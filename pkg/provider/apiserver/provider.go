package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/avitaltamir/cyphernetes/pkg/provider"
	openapi_v3 "github.com/google/gnostic/openapiv3"
	"google.golang.org/protobuf/proto"
)

type APIServerProvider struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	gvrCache      map[string]schema.GroupVersionResource
	gvrCacheMutex sync.RWMutex
	openAPIDoc    *openapi_v3.Document
}

func NewAPIServerProvider() (provider.Provider, error) {
	// First try in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	provider := &APIServerProvider{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		gvrCache:      make(map[string]schema.GroupVersionResource),
	}

	// Initialize the GVR cache
	if err := provider.initGVRCache(); err != nil {
		return nil, fmt.Errorf("error initializing GVR cache: %w", err)
	}

	return provider, nil
}

// Add this method to create a provider with a specific kubeconfig
func NewAPIServerProviderWithConfig(kubeConfig clientcmd.ClientConfig) (provider.Provider, error) {
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	return &APIServerProvider{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		gvrCache:      make(map[string]schema.GroupVersionResource),
	}, nil
}

// Implement Provider interface methods...
func (p *APIServerProvider) GetK8sResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error) {
	gvr, err := p.FindGVR(kind)
	if err != nil {
		return nil, err
	}

	var list *unstructured.UnstructuredList
	if namespace != "" {
		list, err = p.dynamicClient.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{
			FieldSelector: fieldSelector,
			LabelSelector: labelSelector,
		})
	} else {
		list, err = p.dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{
			FieldSelector: fieldSelector,
			LabelSelector: labelSelector,
		})
	}

	if err != nil {
		return nil, err
	}

	// Convert list items to []map[string]interface{}
	var converted []map[string]interface{}
	for _, u := range list.Items {
		converted = append(converted, u.UnstructuredContent())
	}
	return converted, nil
}

// Move the FindGVR implementation from k8s_client.go here
func (p *APIServerProvider) FindGVR(kind string) (schema.GroupVersionResource, error) {
	p.gvrCacheMutex.RLock()
	// Try exact match first
	if gvr, ok := p.gvrCache[kind]; ok {
		p.gvrCacheMutex.RUnlock()
		return gvr, nil
	}
	p.gvrCacheMutex.RUnlock()

	// If not found, try to refresh the cache
	if err := p.initGVRCache(); err != nil {
		return schema.GroupVersionResource{}, err
	}

	p.gvrCacheMutex.RLock()
	defer p.gvrCacheMutex.RUnlock()

	// Try exact match again after refresh
	if gvr, ok := p.gvrCache[kind]; ok {
		return gvr, nil
	}

	// Try case-insensitive lookup and handle variations
	lowerKind := strings.ToLower(kind)
	for k, gvr := range p.gvrCache {
		// Check for:
		// 1. Case-insensitive match of kind
		// 2. Plural form matches resource name
		// 3. Singular form matches resource name
		// 4. Short name matches
		if strings.ToLower(k) == lowerKind || // Case-insensitive kind match
			strings.ToLower(gvr.Resource) == lowerKind || // Plural form
			strings.ToLower(strings.TrimSuffix(gvr.Resource, "s")) == lowerKind || // Singular form
			strings.ToLower(strings.TrimSuffix(gvr.Resource, "es")) == lowerKind || // Singular form
			containsStringIgnoreCase(p.getShortNames(gvr), lowerKind) { // Short name match
			return gvr, nil
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("resource %q not found", kind)
}

// Add helper method to get short names for a GVR
func (p *APIServerProvider) getShortNames(gvr schema.GroupVersionResource) []string {
	resources, err := p.clientset.Discovery().ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		return nil
	}

	for _, r := range resources.APIResources {
		if r.Name == gvr.Resource {
			return r.ShortNames
		}
	}
	return nil
}

// Helper function for case-insensitive string slice contains
func containsStringIgnoreCase(slice []string, str string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, str) {
			return true
		}
	}
	return false
}

// Implement other Provider interface methods...
func (p *APIServerProvider) DeleteK8sResources(kind, name, namespace string) error {
	gvr, err := p.FindGVR(kind)
	if err != nil {
		return err
	}

	var deleteErr error
	if namespace != "" {
		deleteErr = p.dynamicClient.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if deleteErr == nil {
			fmt.Printf("Deleted %s/%s in namespace %s\n", strings.ToLower(kind), name, namespace)
		}
	} else {
		deleteErr = p.dynamicClient.Resource(gvr).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if deleteErr == nil {
			fmt.Printf("Deleted %s/%s\n", strings.ToLower(kind), name)
		}
	}

	return deleteErr
}

func (p *APIServerProvider) CreateK8sResource(kind, name, namespace string, body interface{}) error {
	gvr, err := p.FindGVR(kind)
	if err != nil {
		return err
	}

	unstructuredObj, err := toUnstructured(body)
	if err != nil {
		return err
	}

	// Ensure metadata and name are set
	if unstructuredObj.Object["metadata"] == nil {
		unstructuredObj.Object["metadata"] = map[string]interface{}{}
	}
	metadata := unstructuredObj.Object["metadata"].(map[string]interface{})
	metadata["name"] = name

	if namespace != "" {
		metadata["namespace"] = namespace
		_, err = p.dynamicClient.Resource(gvr).Namespace(namespace).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{})
		if err == nil {
			fmt.Printf("\nCreated %s/%s in namespace %s", strings.ToLower(kind), name, namespace)
		}
	} else {
		_, err = p.dynamicClient.Resource(gvr).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{})
		if err == nil {
			fmt.Printf("\nCreated %s/%s", strings.ToLower(kind), name)
		}
	}

	return err
}

func (p *APIServerProvider) PatchK8sResource(kind, name, namespace string, body interface{}) error {
	gvr, err := p.FindGVR(kind)
	if err != nil {
		return err
	}

	// Convert body to JSON patch format if it's not already
	var patchData []byte
	switch data := body.(type) {
	case []byte:
		patchData = data
	case string:
		patchData = []byte(data)
	default:
		patchData, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("error marshalling patch data: %v", err)
		}
	}

	// Apply the patch
	if namespace != "" {
		_, err = p.dynamicClient.Resource(gvr).Namespace(namespace).Patch(
			context.TODO(),
			name,
			types.JSONPatchType,
			patchData,
			metav1.PatchOptions{},
		)
		if err == nil {
			fmt.Printf("Patched %s/%s in namespace %s\n", strings.ToLower(kind), name, namespace)
		}
	} else {
		_, err = p.dynamicClient.Resource(gvr).Patch(
			context.TODO(),
			name,
			types.JSONPatchType,
			patchData,
			metav1.PatchOptions{},
		)
		if err == nil {
			fmt.Printf("Patched %s/%s\n", strings.ToLower(kind), name)
		}
	}

	return err
}

func (p *APIServerProvider) GetOpenAPIResourceSpecs() (map[string][]string, error) {
	fmt.Print("🔎 fetching resource specs... ")
	if p.openAPIDoc == nil {
		// Get OpenAPI V3 client
		openAPIV3Client := p.clientset.Discovery().OpenAPIV3()

		// Get the OpenAPI V3 paths
		paths, err := openAPIV3Client.Paths()
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve OpenAPI paths: %v", err)
		}

		// Initialize openAPIDoc
		p.openAPIDoc = &openapi_v3.Document{
			Components: &openapi_v3.Components{
				Schemas: &openapi_v3.SchemasOrReferences{
					AdditionalProperties: []*openapi_v3.NamedSchemaOrReference{},
				},
			},
		}

		// Process each group version
		for _, groupVersion := range paths {
			schemaBytes, err := groupVersion.Schema("application/com.github.proto-openapi.spec.v3@v1.0+protobuf")
			if err != nil {
				if strings.Contains(err.Error(), "the backend attempted to redirect this request") {
					continue
				}
				continue
			}

			// Unmarshal into temporary document
			tempDoc := &openapi_v3.Document{}
			err = proto.Unmarshal(schemaBytes, tempDoc)
			if err != nil {
				continue
			}

			// Merge schemas
			if tempDoc.Components != nil && tempDoc.Components.Schemas != nil {
				p.openAPIDoc.Components.Schemas.AdditionalProperties = append(
					p.openAPIDoc.Components.Schemas.AdditionalProperties,
					tempDoc.Components.Schemas.AdditionalProperties...,
				)
			}
		}
	}

	// Process schemas into field paths
	specs := make(map[string][]string)
	if p.openAPIDoc.Components != nil && p.openAPIDoc.Components.Schemas != nil {
		for _, schemaEntry := range p.openAPIDoc.Components.Schemas.AdditionalProperties {
			resourceName := schemaEntry.Name
			schema := schemaEntry.Value.GetSchema()
			if schema != nil {
				visited := make(map[string][]string)
				fields := p.parseSchema(schema, "", visited, "")
				specs[resourceName] = fields
			}
		}
	}

	fmt.Println("done!")
	return specs, nil
}

// Helper function to convert interface{} to *unstructured.Unstructured
func toUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	switch v := obj.(type) {
	case *unstructured.Unstructured:
		return v, nil
	default:
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		var unstructuredObj map[string]interface{}
		if err := json.Unmarshal(data, &unstructuredObj); err != nil {
			return nil, err
		}
		return &unstructured.Unstructured{Object: unstructuredObj}, nil
	}
}

// Add the parseSchema method and its helpers
func (p *APIServerProvider) parseSchema(schema *openapi_v3.Schema, prefix string, visited map[string][]string, parentType string) []string {
	var fields []string

	// Check if we've already visited this schema
	schemaKey := fmt.Sprintf("%p", schema)
	if visitedFields, ok := visited[schemaKey]; ok {
		return visitedFields
	}

	// Mark as visited with empty slice to prevent infinite recursion
	visited[schemaKey] = []string{}

	// If this is a top-level object, look for the ObjectMeta schema
	if prefix == "" {
		metadataSchema := p.resolveReference("#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta")
		if metadataSchema != nil {
			metadataFields := p.parseSchema(metadataSchema, "metadata", visited, "object")
			fields = append(fields, metadataFields...)
		}
	}

	// Handle allOf schemas
	if len(schema.AllOf) > 0 {
		for _, subSchemaOrRef := range schema.AllOf {
			var subSchema *openapi_v3.Schema
			if ref := subSchemaOrRef.GetReference(); ref != nil && ref.XRef != "" {
				subSchema = p.resolveReference(ref.XRef)
			} else {
				subSchema = subSchemaOrRef.GetSchema()
			}

			if subSchema != nil {
				subFields := p.parseSchema(subSchema, prefix, visited, parentType)
				fields = append(fields, subFields...)
			}
		}
	}

	if schema.Type == "object" && schema.Properties != nil {
		for _, prop := range schema.Properties.AdditionalProperties {
			fieldName := prop.Name
			fieldPath := fieldName
			if prefix != "" {
				fieldPath = prefix + "." + fieldName
			}

			// Add the current field path
			fields = append(fields, fieldPath)

			// Handle reference or schema
			var propSchema *openapi_v3.Schema
			if ref := prop.Value.GetReference(); ref != nil && ref.XRef != "" {
				propSchema = p.resolveReference(ref.XRef)
			} else {
				propSchema = prop.Value.GetSchema()
			}

			if propSchema != nil {
				// Handle arrays - add this section
				if propSchema.Type == "array" && propSchema.Items != nil && len(propSchema.Items.SchemaOrReference) > 0 {
					var itemSchema *openapi_v3.Schema
					if ref := propSchema.Items.SchemaOrReference[0].GetReference(); ref != nil && ref.XRef != "" {
						itemSchema = p.resolveReference(ref.XRef)
					} else {
						itemSchema = propSchema.Items.SchemaOrReference[0].GetSchema()
					}

					if itemSchema != nil {
						arrayPath := fieldPath + "[]"
						fields = append(fields, arrayPath)
						// Recursively process array item schema
						arrayFields := p.parseSchema(itemSchema, arrayPath, visited, "array")
						fields = append(fields, arrayFields...)
					}
				}

				// Continue with normal nested field processing
				nestedFields := p.parseSchema(propSchema, fieldPath, visited, "object")
				fields = append(fields, nestedFields...)
			}
		}
	}

	// Handle array items
	if schema.Type == "array" && schema.Items != nil && len(schema.Items.SchemaOrReference) > 0 {
		var itemSchema *openapi_v3.Schema
		if ref := schema.Items.SchemaOrReference[0].GetReference(); ref != nil && ref.XRef != "" {
			itemSchema = p.resolveReference(ref.XRef)
			if itemSchema == nil {
				itemSchema = schema.Items.SchemaOrReference[0].GetSchema()
			}
		} else {
			itemSchema = schema.Items.SchemaOrReference[0].GetSchema()
		}

		if itemSchema != nil {
			arrayPath := prefix + "[]"
			fields = append(fields, arrayPath)
			arrayFields := p.parseSchema(itemSchema, arrayPath, visited, "array")
			fields = append(fields, arrayFields...)
		}
	}

	// Handle additionalProperties (maps)
	if schema.AdditionalProperties != nil {
		if addPropSchemaOrRef := schema.AdditionalProperties.GetSchemaOrReference(); addPropSchemaOrRef != nil {
			var addPropSchema *openapi_v3.Schema
			if ref := addPropSchemaOrRef.GetReference(); ref != nil && ref.XRef != "" {
				addPropSchema = p.resolveReference(ref.XRef)
			} else {
				addPropSchema = addPropSchemaOrRef.GetSchema()
			}

			if addPropSchema != nil {
				mapPath := prefix + "{}"
				nestedFields := p.parseSchema(addPropSchema, mapPath, visited, parentType)
				fields = append(fields, nestedFields...)
			}
		}
	}

	// Store the fields in visited map
	visited[schemaKey] = fields
	return fields
}

func (p *APIServerProvider) resolveReference(ref string) *openapi_v3.Schema {
	// Remove the #/components/schemas/ prefix if present
	ref = strings.TrimPrefix(ref, "#/components/schemas/")

	if p.openAPIDoc == nil || p.openAPIDoc.Components == nil || p.openAPIDoc.Components.Schemas == nil {
		return nil
	}

	// Look for the referenced schema
	for _, schemaEntry := range p.openAPIDoc.Components.Schemas.AdditionalProperties {
		if schemaEntry.Name == ref {
			if schema := schemaEntry.Value.GetSchema(); schema != nil {
				return schema
			}
			if ref := schemaEntry.Value.GetReference(); ref != nil {
				// Handle nested references
				return p.resolveReference(ref.XRef)
			}
		}
	}

	return nil
}

// Add this method to implement the Provider interface
func (p *APIServerProvider) CreateProviderForContext(context string) (provider.Provider, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: context,
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Create new provider with the context's config
	newProvider, err := NewAPIServerProviderWithConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	// Initialize the GVR cache for the new context
	apiProvider := newProvider.(*APIServerProvider)
	if err := apiProvider.initGVRCache(); err != nil {
		return nil, err
	}

	return apiProvider, nil
}

// Add these methods to APIServerProvider

func (p *APIServerProvider) GetDiscoveryClient() (discovery.DiscoveryInterface, error) {
	return p.clientset.Discovery(), nil
}

func (p *APIServerProvider) GetClientset() (kubernetes.Interface, error) {
	return p.clientset, nil
}

func (p *APIServerProvider) GetGVRCache() (map[string]schema.GroupVersionResource, error) {
	// Initialize the cache if it's empty
	if len(p.gvrCache) == 0 {
		err := p.initGVRCache()
		if err != nil {
			return nil, fmt.Errorf("error initializing GVR cache: %w", err)
		}
	}
	return p.gvrCache, nil
}

// Add this helper method if not already present
func (p *APIServerProvider) initGVRCache() error {
	if p.gvrCache == nil {
		p.gvrCache = make(map[string]schema.GroupVersionResource)
	}

	resources, err := p.clientset.Discovery().ServerPreferredResources()
	if err != nil {
		return fmt.Errorf("error getting server resources: %w", err)
	}

	for _, list := range resources {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for _, r := range list.APIResources {
			if strings.Contains(r.Name, "/") {
				continue
			}

			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: r.Name,
			}

			// Store with kind as key
			p.gvrCache[r.Kind] = gvr
			// Store with resource name (plural) as key
			p.gvrCache[r.Name] = gvr
			// Store with singular name as key
			if r.SingularName != "" {
				p.gvrCache[r.SingularName] = gvr
			}
			// Store with short names as keys
			for _, shortName := range r.ShortNames {
				p.gvrCache[shortName] = gvr
			}
		}
	}

	return nil
}

func (p *APIServerProvider) GetDynamicClient() (dynamic.Interface, error) {
	return p.dynamicClient, nil
}
