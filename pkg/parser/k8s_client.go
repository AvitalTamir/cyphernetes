package parser

import (
	"context"
	"fmt"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	openapi_v3 "github.com/google/gnostic/openapiv3"
	"google.golang.org/protobuf/proto"
)

var (
	executorInstance *QueryExecutor
	contextExecutors map[string]*QueryExecutor
	executorsLock    sync.RWMutex
	openAPIDoc       *openapi_v3.Document
	openAPIDocMutex  sync.RWMutex
	once             sync.Once
)

// Make resourceSpecs accessible outside the package
var ResourceSpecs = make(map[string][]string)

// func init() {
// 	initResourceSpecs()
// }

func InitResourceSpecs() {
	specs, err := GetOpenAPIResourceSpecs()
	if err != nil {
		fmt.Println("Error fetching resource specs:", err)
		return
	}
	ResourceSpecs = specs
	// Initialize relationships after specs are loaded
	initializeRelationships()
}

func GetQueryExecutorInstance() *QueryExecutor {
	once.Do(func() {
		executor, err := NewQueryExecutor()
		if err != nil {
			// Handle error
			fmt.Println("Error creating QueryExecutor instance:", err)
			return
		}
		executorInstance = executor
		contextExecutors = make(map[string]*QueryExecutor)
	})
	return executorInstance
}

func GetContextQueryExecutor(context string) (*QueryExecutor, error) {
	executorsLock.RLock()
	if executor, exists := contextExecutors[context]; exists {
		executorsLock.RUnlock()
		return executor, nil
	}
	executorsLock.RUnlock()

	// Create new executor for this context
	executorsLock.Lock()
	defer executorsLock.Unlock()

	// Double-check after acquiring write lock
	if executor, exists := contextExecutors[context]; exists {
		return executor, nil
	}

	// Create config for specific context
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: context,
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create config for context %s: %v", context, err)
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset for context %s: %v", context, err)
	}

	// Create the dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client for context %s: %v", context, err)
	}

	executor := &QueryExecutor{
		Clientset:      clientset,
		DynamicClient:  dynamicClient,
		requestChannel: make(chan *apiRequest),
		semaphore:      make(chan struct{}, 1),
	}

	go executor.processRequests()

	if contextExecutors == nil {
		contextExecutors = make(map[string]*QueryExecutor)
	}
	contextExecutors[context] = executor
	return executor, nil
}

type QueryExecutor struct {
	Clientset      *kubernetes.Clientset
	DynamicClient  dynamic.Interface
	requestChannel chan *apiRequest
	semaphore      chan struct{}
}

type apiRequest struct {
	kind          string
	fieldSelector string
	labelSelector string
	namespace     string
	responseChan  chan *apiResponse
}

type apiResponse struct {
	list *unstructured.UnstructuredList
	err  error
}

func NewQueryExecutor() (*QueryExecutor, error) {
	var config *rest.Config
	var err error

	// First, try to use in-cluster config
	config, err = rest.InClusterConfig()
	if err != nil {
		// If that fails, use the kubeconfig file(s)
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create config: not found in $KUBECONFIG, ~/.kube/config, or in-cluster")
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %v", err)
	}

	// Create the dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client: %v", err)
	}

	// Initialize the semaphore with a desired concurrency level
	semaphore := make(chan struct{}, 1) // Set to '1' for single concurrent request

	executor := &QueryExecutor{
		Clientset:      clientset,
		DynamicClient:  dynamicClient,
		requestChannel: make(chan *apiRequest), // Unbuffered channel
		semaphore:      semaphore,
	}

	go executor.processRequests()

	return executor, nil
}

func (q *QueryExecutor) GetClientset() kubernetes.Interface {
	return q.Clientset
}

func (q *QueryExecutor) GetDynamicClient() dynamic.Interface {
	return q.DynamicClient
}

func (q *QueryExecutor) processRequests() {
	for request := range q.requestChannel {
		q.semaphore <- struct{}{} // Acquire a token
		list, err := q.fetchResources(request.kind, request.fieldSelector, request.labelSelector, request.namespace)
		<-q.semaphore // Release the token
		request.responseChan <- &apiResponse{list: &list, err: err}
	}
}

func (q *QueryExecutor) getK8sResources(kind string, fieldSelector string, labelSelector string, namespace string) (*unstructured.UnstructuredList, error) {
	responseChan := make(chan *apiResponse)
	q.requestChannel <- &apiRequest{
		kind:          kind,
		fieldSelector: fieldSelector,
		labelSelector: labelSelector,
		namespace:     namespace,
		responseChan:  responseChan,
	}

	response := <-responseChan
	return response.list, response.err
}

func (q *QueryExecutor) fetchResources(kind string, fieldSelector string, labelSelector string, namespace string) (unstructured.UnstructuredList, error) {
	labelSelector = strings.ReplaceAll(labelSelector, "\"", "")
	// Use discovery client to find the GVR for the given kind
	gvr, err := FindGVR(q.Clientset, kind)
	if err != nil {
		var emptyList unstructured.UnstructuredList
		return emptyList, err
	}

	// Use dynamic client to list resources
	logDebug("Listing resources of kind:", kind, "with fieldSelector:", fieldSelector, "and labelSelector:", labelSelector)
	labelSelectorParsed, err := metav1.ParseToLabelSelector(labelSelector)
	if err != nil {
		fmt.Println("Error parsing label selector: ", err)
		var emptyList unstructured.UnstructuredList
		return emptyList, err
	}
	labelMap, err := metav1.LabelSelectorAsSelector(labelSelectorParsed)
	if err != nil {
		fmt.Println("Error converting label selector to label map: ", err)
		var emptyList unstructured.UnstructuredList
		return emptyList, err
	}

	list, err := q.DynamicClient.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: fieldSelector,
		LabelSelector: labelMap.String(),
	})
	if err != nil {
		fmt.Println("Error getting list of resources: ", err)
		var emptyList unstructured.UnstructuredList
		return emptyList, err
	}
	return *list, err
}

var GvrCache = make(map[string]schema.GroupVersionResource)
var GvrCacheMutex sync.RWMutex
var apiResourceListCache []*metav1.APIResourceList

func FindGVR(clientset *kubernetes.Clientset, resourceId string) (schema.GroupVersionResource, error) {
	normalizedIdentifier := strings.ToLower(resourceId)

	// Check if the GVR is already in the cache
	GvrCacheMutex.RLock()
	if gvr, ok := GvrCache[normalizedIdentifier]; ok {
		GvrCacheMutex.RUnlock()
		return gvr, nil
	}
	GvrCacheMutex.RUnlock()

	// GVR not in cache, find it using discovery
	if apiResourceListCache == nil {
		discoveryClient := clientset.Discovery()
		apiResourceList, err := discoveryClient.ServerPreferredResources()
		if err != nil {
			return schema.GroupVersionResource{}, err
		}
		apiResourceListCache = apiResourceList
	}

	for _, apiResource := range apiResourceListCache {
		for _, resource := range apiResource.APIResources {
			if strings.EqualFold(resource.Name, normalizedIdentifier) ||
				strings.EqualFold(resource.Kind, resourceId) ||
				containsIgnoreCase(resource.ShortNames, normalizedIdentifier) {

				gv, err := schema.ParseGroupVersion(apiResource.GroupVersion)
				if err != nil {
					return schema.GroupVersionResource{}, err
				}
				gvr := gv.WithResource(resource.Name)

				// Update the cache
				GvrCacheMutex.Lock()
				GvrCache[normalizedIdentifier] = gvr
				GvrCacheMutex.Unlock()

				return gvr, nil
			}
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("resource identifier not found: %s", resourceId)
}

// Helper function to check if a slice contains a string, case-insensitive
func containsIgnoreCase(slice []string, str string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, str) {
			return true
		}
	}
	return false
}

func FetchAndCacheGVRs(clientset *kubernetes.Clientset) error {
	discoveryClient := clientset.Discovery()
	apiResourceList, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return err
	}

	for _, apiResourceGroup := range apiResourceList {
		gv, err := schema.ParseGroupVersion(apiResourceGroup.GroupVersion)
		if err != nil {
			// Handle error or continue with the next group
			continue
		}

		for _, resource := range apiResourceGroup.APIResources {
			gvr := gv.WithResource(resource.Name)
			gvrKey := resource.Name // Or use a more specific key if needed
			GvrCache[gvrKey] = gvr
		}
	}

	return nil
}

var (
	resourceSpecsCache map[string][]string
	resourceSpecsMutex sync.Mutex
)

// GetOpenAPIResourceSpecs initializes and caches the resource specs
func GetOpenAPIResourceSpecs() (map[string][]string, error) {
	resourceSpecsMutex.Lock()
	defer resourceSpecsMutex.Unlock()

	if resourceSpecsCache != nil {
		return resourceSpecsCache, nil
	}

	specs, err := fetchResourceSpecsFromOpenAPI()
	if err != nil {
		return nil, err
	}

	resourceSpecsCache = specs
	return specs, nil
}

// fetchResourceSpecsFromOpenAPI fetches and parses the OpenAPI V3 schemas
func fetchResourceSpecsFromOpenAPI() (map[string][]string, error) {
	if !CleanOutput {
		fmt.Print("🔎 fetching resource specs... ")
	}
	openAPIDocMutex.Lock()
	defer openAPIDocMutex.Unlock()
	specs := make(map[string][]string)

	executorInstance := GetQueryExecutorInstance()

	if openAPIDoc == nil {

		// Use the existing clientset from QueryExecutor
		discoveryClient := executorInstance.Clientset.Discovery()

		// Get OpenAPI V3 client
		openAPIV3Client := discoveryClient.OpenAPIV3()

		// Get the OpenAPI V3 paths
		paths, err := openAPIV3Client.Paths()
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve OpenAPI paths: %v", err)
		}

		// Initialize openAPIDoc with empty Components before the loop
		openAPIDoc = &openapi_v3.Document{
			Components: &openapi_v3.Components{
				Schemas: &openapi_v3.SchemasOrReferences{
					AdditionalProperties: []*openapi_v3.NamedSchemaOrReference{},
				},
			},
		}

		for _, groupVersion := range paths {
			schemaBytes, err := groupVersion.Schema("application/com.github.proto-openapi.spec.v3@v1.0+protobuf")
			if err != nil {
				if strings.Contains(err.Error(), "the backend attempted to redirect this request") {
					continue
				}
				fmt.Printf("Error retrieving schema for group version %s: %v\n", groupVersion, err)
				continue
			}

			// Unmarshal into a temporary document to prevent overwriting openAPIDoc
			tempDoc := &openapi_v3.Document{}
			err = proto.Unmarshal(schemaBytes, tempDoc)
			if err != nil {
				fmt.Printf("Error unmarshaling OpenAPI document for group version %s: %v\n", groupVersion, err)
				continue
			}

			// Merge tempDoc.Components.Schemas into openAPIDoc.Components.Schemas
			if tempDoc.Components != nil && tempDoc.Components.Schemas != nil {
				openAPIDoc.Components.Schemas.AdditionalProperties = append(openAPIDoc.Components.Schemas.AdditionalProperties, tempDoc.Components.Schemas.AdditionalProperties...)
			}

			// Process the schemas from tempDoc
			if tempDoc.Components != nil && tempDoc.Components.Schemas != nil {
				for _, schemaEntry := range tempDoc.Components.Schemas.AdditionalProperties {
					resourceName := schemaEntry.Name
					schema := schemaEntry.Value.GetSchema()
					if schema != nil {
						// Pass the visited map to prevent infinite recursion
						visited := make(map[string][]string) // Initialize the visited map
						fields := parseSchema(schema, "", visited, "")
						specs[resourceName] = fields
					}
				}
			}
		}
	}
	if !CleanOutput {
		fmt.Println("done!")
	}
	return specs, nil
}

// parseSchema recursively extracts field paths from the schema
func parseSchema(schema *openapi_v3.Schema, prefix string, visited map[string][]string, parent string) []string {
	fields := []string{}
	if schema == nil {
		return fields
	}

	// Check if the schema has a "kind" field and is a valid GVR
	if parent == "" && schema.SpecificationExtension != nil {
		for _, ext := range schema.SpecificationExtension {
			if ext.Name == "x-kubernetes-group-version-kind" {
				kindYaml := ext.Value.Yaml
				kind := ""
				lines := strings.Split(kindYaml, "\n")
				for _, line := range lines {
					if strings.Contains(line, "kind:") {
						kind = strings.Split(line, ":")[1]
						kind = strings.TrimSpace(kind)
					}
				}
				if kind != "" {
					gvr, err := FindGVR(executorInstance.Clientset, kind)
					if err == nil {
						fields = append(fields, processSchema(schema, prefix, visited, gvr.Resource)...)
					} else {
						fields = append(fields, processSchema(schema, prefix, visited, parent)...)
					}
				} else {
					fields = append(fields, processSchema(schema, prefix, visited, parent)...)
				}
			}
		}
	} else if prefix != "" {
		fields = append(fields, processSchema(schema, prefix, visited, parent)...)
	}

	return fields
}

func processSchema(schema *openapi_v3.Schema, prefix string, visited map[string][]string, parent string) []string {
	// Check if the schema has been cached
	// Create a unique key that includes both the schema pointer and kind
	uniqueKey := fmt.Sprintf("%p", schema)

	// Check if the schema has been cached
	if visitedFields, ok := visited[uniqueKey]; ok {
		return visitedFields
	}

	logDebug(fmt.Sprintf("Processing schema: %s\n", schema))

	fields := []string{}

	// Handle properties
	if schema.Properties != nil {
		for _, prop := range schema.Properties.AdditionalProperties {
			fieldName := prop.Name
			fullName := fieldName
			if prefix != "" {
				fullName = fmt.Sprintf("%s.%s", prefix, fieldName)
			}
			fields = append(fields, fullName)
			visited[uniqueKey] = fields

			var nestedSchema *openapi_v3.Schema
			if schemaOrRef := prop.Value; schemaOrRef != nil {
				if ref := schemaOrRef.GetReference(); ref != nil && ref.XRef != "" {
					nestedSchema = resolveReference(ref.XRef, parent, fullName)
					if nestedSchema == nil {
						logError(fmt.Sprintf("Failed to resolve reference for field: %s\n", fullName))
						continue
					}
				} else if nested := schemaOrRef.GetSchema(); nested != nil {
					nestedSchema = nested
				}
			}

			if nestedSchema != nil {
				nestedFields := parseSchema(nestedSchema, fullName, visited, parent)
				if len(nestedFields) > 0 {
					fields = append(fields, nestedFields...)
					visited[uniqueKey] = fields
				}
			}
		}
	}

	// Handle allOf, oneOf, anyOf if present
	if len(schema.AllOf) > 0 {
		for _, subSchemaOrRef := range schema.AllOf {
			var subSchema *openapi_v3.Schema
			if ref := subSchemaOrRef.GetReference(); ref != nil && ref.XRef != "" {
				subSchema = resolveReference(ref.XRef, parent, prefix)
				if subSchema == nil {
					logError(fmt.Sprintf("Failed to resolve reference in allOf: %s\n", ref.XRef))
					continue
				}
			} else if nested := subSchemaOrRef.GetSchema(); nested != nil {
				subSchema = nested
			}

			if subSchema != nil {
				subFields := parseSchema(subSchema, prefix, visited, parent)
				fields = append(fields, subFields...)
				visited[uniqueKey] = subFields
			}
		}
	}

	// Handle array items
	if schema.Type == "array" && schema.Items != nil && len(schema.Items.SchemaOrReference) > 0 {
		itemSchemaOrRef := schema.Items.SchemaOrReference[0]
		if itemSchemaOrRef != nil {
			var itemSchema *openapi_v3.Schema
			if ref := itemSchemaOrRef.GetReference(); ref != nil && ref.XRef != "" {
				itemSchema = resolveReference(ref.XRef, parent, prefix)
				if itemSchema == nil {
					logError(fmt.Sprintf("Failed to resolve reference for array items at: %s\n", prefix))
					return fields
				}
			} else if nested := itemSchemaOrRef.GetSchema(); nested != nil {
				itemSchema = nested
			}

			if itemSchema != nil {
				arrayPrefix := prefix + "[]"
				nestedFields := parseSchema(itemSchema, arrayPrefix, visited, parent)
				fields = append(fields, nestedFields...)
				visited[uniqueKey] = fields
			}
		}
	}

	// Handle additionalProperties (for maps)
	if schema.AdditionalProperties != nil {
		if addPropSchemaOrRef := schema.AdditionalProperties.GetSchemaOrReference(); addPropSchemaOrRef != nil {
			var addPropSchema *openapi_v3.Schema
			if ref := addPropSchemaOrRef.GetReference(); ref != nil && ref.XRef != "" {
				addPropSchema = resolveReference(ref.XRef, parent, prefix)
			} else if nested := addPropSchemaOrRef.GetSchema(); nested != nil {
				addPropSchema = nested
			}

			if addPropSchema != nil {
				addPropPrefix := prefix + "{}"
				nestedFields := parseSchema(addPropSchema, addPropPrefix, visited, parent)
				visited[uniqueKey] = nestedFields
				fields = append(fields, nestedFields...)
			}
		}
	}
	if visited[uniqueKey] == nil {
		visited[uniqueKey] = fields
	}

	return fields
}

// resolveReference resolves a $ref string to its corresponding schema
func resolveReference(ref string, parent string, path string) *openapi_v3.Schema {
	// Example ref format: "#/components/schemas/Pod"
	refParts := strings.Split(ref, "/")
	if len(refParts) < 4 {
		logError(fmt.Sprintf("Invalid ref format: %s\n", ref))
		return nil // Invalid ref format
	}
	schemaName := refParts[len(refParts)-1]

	if openAPIDoc == nil || openAPIDoc.Components == nil || openAPIDoc.Components.Schemas == nil {
		logError(fmt.Sprintln("openAPIDoc or its Components/Schemas are nil"))
		return nil
	}

	for _, schemaEntry := range openAPIDoc.Components.Schemas.AdditionalProperties {
		if schemaEntry.Name == schemaName {
			// Check for relationship
			if parent != "" {
				kind := extractKindFromSchemaName(schemaName)
				gvr, err := FindGVR(executorInstance.Clientset, kind)
				if err == nil {
					createRelationshipRule(parent, gvr.Resource, path)
				}
			}
			logDebug(fmt.Sprintf("Resolved reference %s to schema %s\n", ref, schemaName))
			return schemaEntry.Value.GetSchema()
		}
	}

	return nil
}

func extractKindFromSchemaName(schemaName string) string {
	parts := strings.Split(schemaName, ".")
	if len(parts) > 0 {
		return strings.ToLower(parts[len(parts)-1])
	}
	return ""
}

func createRelationshipRule(parent string, schemaName string, path string) {
	// first check if the relationship between the 2 kinds already exists
	rule, err := findRuleByKinds(schemaName, parent)
	fieldA := "$." + path + ".metadata.name"
	fieldB := "$.metadata.name"
	kindA := schemaName
	kindB := parent
	if err != nil {
		relationshipRule := RelationshipRule{
			KindA:        kindA,
			KindB:        kindB,
			Relationship: RelationshipType(fmt.Sprintf("%s_REFERENCES_%s", kindB, kindA)),
			MatchCriteria: []MatchCriterion{
				{
					FieldA:         fieldA,
					FieldB:         fieldB,
					ComparisonType: ExactMatch,
				},
			},
		}
		relationshipRules = append(relationshipRules, relationshipRule)
	} else if len(rule.MatchCriteria) > 0 {
		if rule.KindA == kindB && rule.KindB == kindA {
			fieldA = "$.metadata.name"
			fieldB = "$." + path + ".metadata.name"
		}

		rule.MatchCriteria = append(rule.MatchCriteria, MatchCriterion{
			FieldA:         fieldA,
			FieldB:         fieldB,
			ComparisonType: ExactMatch,
		})
	}
}
