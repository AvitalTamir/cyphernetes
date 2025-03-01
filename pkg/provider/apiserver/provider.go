package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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

type APIServerProviderConfig struct {
	Clientset     kubernetes.Interface
	DynamicClient dynamic.Interface
	DryRun        bool
}

type APIServerProvider struct {
	clientset       kubernetes.Interface
	dynamicClient   dynamic.Interface
	gvrCache        map[string]schema.GroupVersionResource
	gvrCacheMutex   sync.RWMutex
	openAPIDoc      *openapi_v3.Document
	requestChannel  chan *apiRequest
	semaphore       chan struct{}
	resourceMutex   sync.RWMutex
	dryRun          bool
	namespacedCache map[string]bool
	namespacedMutex sync.RWMutex
}

type apiRequest struct {
	kind          string
	fieldSelector string
	labelSelector string
	namespace     string
	responseChan  chan *apiResponse
}

type apiResponse struct {
	result interface{}
	err    error
}

func NewAPIServerProvider() (provider.Provider, error) {
	return NewAPIServerProviderWithOptions(&APIServerProviderConfig{})
}

func NewAPIServerProviderWithOptions(config *APIServerProviderConfig) (provider.Provider, error) {
	var err error
	clientset := config.Clientset
	dynamicClient := config.DynamicClient

	// If clients are not provided, create them
	if clientset == nil || dynamicClient == nil {
		// First try in-cluster config
		var restConfig *rest.Config
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			// Fall back to kubeconfig
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			configOverrides := &clientcmd.ConfigOverrides{}
			kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
			restConfig, err = kubeConfig.ClientConfig()
			if err != nil {
				return nil, fmt.Errorf("failed to create config: %v", err)
			}
		}

		if clientset == nil {
			clientset, err = kubernetes.NewForConfig(restConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to create clientset: %v", err)
			}
		}

		if dynamicClient == nil {
			dynamicClient, err = dynamic.NewForConfig(restConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to create dynamic client: %v", err)
			}
		}
	}

	provider := &APIServerProvider{
		clientset:       clientset,
		dynamicClient:   dynamicClient,
		gvrCache:        make(map[string]schema.GroupVersionResource),
		requestChannel:  make(chan *apiRequest),
		semaphore:       make(chan struct{}, 1),
		dryRun:          config.DryRun,
		namespacedCache: make(map[string]bool),
	}

	if config.DryRun {
		fmt.Println("Provider initialized in dry-run mode")
	}

	// Start the request processor
	go provider.processRequests()

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

	provider := &APIServerProvider{
		clientset:      clientset,
		dynamicClient:  dynamicClient,
		gvrCache:       make(map[string]schema.GroupVersionResource),
		requestChannel: make(chan *apiRequest),
		semaphore:      make(chan struct{}, 1),
	}

	// Start the request processor
	go provider.processRequests()

	// Initialize the GVR cache
	if err := provider.initGVRCache(); err != nil {
		return nil, fmt.Errorf("error initializing GVR cache: %w", err)
	}

	return provider, nil
}

// Implement Provider interface methods...
func (p *APIServerProvider) GetK8sResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error) {
	responseChan := make(chan *apiResponse)
	p.requestChannel <- &apiRequest{
		kind:          kind,
		fieldSelector: fieldSelector,
		labelSelector: labelSelector,
		namespace:     namespace,
		responseChan:  responseChan,
	}

	response := <-responseChan
	return response.result, response.err
}

func (p *APIServerProvider) processRequests() {
	for request := range p.requestChannel {
		p.semaphore <- struct{}{} // Acquire token
		time.Sleep(10 * time.Millisecond)
		list, err := p.fetchResources(request.kind, request.fieldSelector, request.labelSelector, request.namespace)
		<-p.semaphore // Release token
		request.responseChan <- &apiResponse{result: list, err: err}
	}
}

func (p *APIServerProvider) fetchResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error) {
	p.resourceMutex.RLock()
	defer p.resourceMutex.RUnlock()

	gvr, err := p.FindGVR(kind)
	if err != nil {
		return nil, err
	}

	isNamespaced, err := p.isNamespacedResource(gvr)
	if err != nil {
		return nil, err
	}

	var list *unstructured.UnstructuredList
	if namespace != "" && isNamespaced {
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

	var converted []map[string]interface{}
	for _, u := range list.Items {
		converted = append(converted, u.UnstructuredContent())
	}
	return converted, nil
}

// Move the FindGVR implementation from k8s_client.go here
func (p *APIServerProvider) FindGVR(kind string) (schema.GroupVersionResource, error) {
	if kind == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid resource kind: kind cannot be empty")
	}

	p.gvrCacheMutex.RLock()
	defer p.gvrCacheMutex.RUnlock()

	// Use maps to deduplicate matches
	uniqueGVRs := make(map[string]schema.GroupVersionResource)
	uniqueOptions := make(map[string]bool)

	// Special handling for core.* prefix
	if strings.HasPrefix(strings.ToLower(kind), "core.") {
		resourceName := strings.TrimPrefix(kind, "core.")

		// Look for exact match in core group
		for k, gvr := range p.gvrCache {
			if gvr.Group == "" && // core group has empty string as group
				(strings.EqualFold(k, resourceName) ||
					strings.EqualFold(gvr.Resource, resourceName) ||
					strings.EqualFold(strings.TrimSuffix(gvr.Resource, "s"), resourceName)) {
				return gvr, nil
			}
		}
		return schema.GroupVersionResource{}, fmt.Errorf("resource %q not found in core group", resourceName)
	}

	// If kind contains dots (but not starting with core.), treat it as a fully qualified name
	if strings.Contains(kind, ".") {
		// Try exact match only
		if gvr, ok := p.gvrCache[kind]; ok {
			return gvr, nil
		}
		return schema.GroupVersionResource{}, fmt.Errorf("resource %q not found", kind)
	}

	// For non-fully-qualified names, try all the matching strategies
	// Try exact match first
	if gvr, ok := p.gvrCache[kind]; ok {
		key := fmt.Sprintf("%s/%s", gvr.Resource, gvr.Group)
		uniqueGVRs[key] = gvr
		if gvr.Group == "" {
			uniqueOptions["core."+gvr.Resource] = true
		} else {
			uniqueOptions[gvr.Resource+"."+gvr.Group] = true
		}
	}

	// Try case-insensitive lookup
	lowerKind := strings.ToLower(kind)
	for k, gvr := range p.gvrCache {
		if strings.ToLower(k) == lowerKind || // Case-insensitive kind match
			strings.ToLower(gvr.Resource) == lowerKind || // Plural form
			strings.ToLower(strings.TrimSuffix(gvr.Resource, "s")) == lowerKind || // Singular form
			strings.ToLower(strings.TrimSuffix(gvr.Resource, "es")) == lowerKind || // Singular form
			(strings.HasSuffix(gvr.Resource, "ies") && strings.ToLower(strings.TrimSuffix(gvr.Resource, "ies")+"y") == lowerKind) { // Handle -ies to -y conversion
			key := fmt.Sprintf("%s/%s", gvr.Resource, gvr.Group)
			uniqueGVRs[key] = gvr
			if gvr.Group == "" {
				uniqueOptions["core."+gvr.Resource] = true
			} else {
				uniqueOptions[gvr.Resource+"."+gvr.Group] = true
			}
		}
	}

	if len(uniqueGVRs) > 1 {
		var options []string
		for option := range uniqueOptions {
			options = append(options, option)
		}
		sort.Strings(options)
		return schema.GroupVersionResource{}, fmt.Errorf("ambiguous resource kind %q found. Please specify one of:\n%s",
			kind, strings.Join(options, "\n"))
	}

	if len(uniqueGVRs) == 1 {
		for _, gvr := range uniqueGVRs {
			return gvr, nil
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("resource %q not found", kind)
}

// Implement other Provider interface methods...
func (p *APIServerProvider) DeleteK8sResources(kind, name, namespace string) error {
	p.resourceMutex.Lock()
	defer p.resourceMutex.Unlock()

	gvr, err := p.FindGVR(kind)
	if err != nil {
		return err
	}

	isNamespaced, err := p.isNamespacedResource(gvr)
	if err != nil {
		return err
	}

	var deleteOpts metav1.DeleteOptions
	if p.dryRun {
		deleteOpts.DryRun = []string{metav1.DryRunAll}
	}

	var deleteErr error
	if namespace != "" && isNamespaced {
		deleteErr = p.dynamicClient.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, deleteOpts)
		if deleteErr == nil {
			if p.dryRun {
				fmt.Printf("Dry run mode: would delete %s/%s\n", strings.ToLower(kind), name)
			} else {
				fmt.Printf("Deleted %s/%s in namespace %s\n", strings.ToLower(kind), name, namespace)
			}
		}
	} else {
		deleteErr = p.dynamicClient.Resource(gvr).Delete(context.TODO(), name, deleteOpts)
		if deleteErr == nil {
			fmt.Printf("Deleted %s/%s\n", strings.ToLower(kind), name)
		}
	}

	return deleteErr
}

func (p *APIServerProvider) CreateK8sResource(kind, name, namespace string, body interface{}) error {
	p.resourceMutex.Lock()
	defer p.resourceMutex.Unlock()

	gvr, err := p.FindGVR(kind)
	if err != nil {
		return err
	}

	isNamespaced, err := p.isNamespacedResource(gvr)
	if err != nil {
		return err
	}

	unstructuredObj, err := toUnstructured(body)
	if err != nil {
		return err
	}

	if unstructuredObj.Object["metadata"] == nil {
		unstructuredObj.Object["metadata"] = map[string]interface{}{}
	}
	metadata := unstructuredObj.Object["metadata"].(map[string]interface{})
	metadata["name"] = name

	createOpts := metav1.CreateOptions{}
	if p.dryRun {
		createOpts.DryRun = []string{metav1.DryRunAll}
	}

	if namespace != "" && isNamespaced {
		metadata["namespace"] = namespace
		_, err = p.dynamicClient.Resource(gvr).Namespace(namespace).Create(context.TODO(), unstructuredObj, createOpts)
		if err == nil {
			if p.dryRun {
				fmt.Printf("\nDry run mode: would create %s/%s", strings.ToLower(kind), name)
			} else {
				fmt.Printf("\nCreated %s/%s in namespace %s", strings.ToLower(kind), name, namespace)
			}
		}
	} else {
		_, err = p.dynamicClient.Resource(gvr).Create(context.TODO(), unstructuredObj, createOpts)
		if err == nil {
			fmt.Printf("\nCreated %s/%s", strings.ToLower(kind), name)
		}
	}

	return err
}

func (p *APIServerProvider) PatchK8sResource(kind, name, namespace string, patchJSON []byte) error {
	fmt.Printf("\n==== PatchK8sResource ====\n")
	fmt.Printf("Kind: %s, Name: %s, Namespace: %s\n", kind, name, namespace)
	fmt.Printf("Patch JSON: %s\n", string(patchJSON))

	gvr, err := p.FindGVR(kind)
	if err != nil {
		fmt.Printf("Error finding GVR: %v\n", err)
		return err
	}

	fmt.Printf("Found GVR: %+v\n", gvr)

	// Parse the patch to handle errors better
	var patches []interface{}
	if err := json.Unmarshal(patchJSON, &patches); err != nil {
		fmt.Printf("Error unmarshalling patch JSON: %v\n", err)
		return fmt.Errorf("invalid patch JSON: %v", err)
	}

	fmt.Printf("Parsed %d patch operations\n", len(patches))

	// Check if this is a patch for metadata.annotations or metadata.labels
	// If so, we'll use a strategic merge patch instead of JSON Patch
	if len(patches) == 2 {
		testPatch, ok1 := patches[0].(map[string]interface{})
		addPatch, ok2 := patches[1].(map[string]interface{})

		if ok1 && ok2 &&
			testPatch["op"] == "test" && addPatch["op"] == "add" {

			testPath, testOk := testPatch["path"].(string)
			addPath, addOk := addPatch["path"].(string)

			// Check for container resources patch
			if testOk && addOk && testPath == "/spec/template/spec/containers" &&
				strings.HasPrefix(addPath, "/spec/template/spec/containers/") {

				fmt.Printf("Detected container patch, using strategic merge patch\n")

				// Extract the container index and property path
				containerPathPattern := regexp.MustCompile(`/spec/template/spec/containers/(\d+)(/.*)?`)
				matches := containerPathPattern.FindStringSubmatch(addPath)

				if len(matches) >= 2 {
					containerIndex := matches[1]
					propertyPath := ""
					if len(matches) >= 3 {
						propertyPath = matches[2]
					}

					// Build a strategic merge patch for the container
					mergePatch := map[string]interface{}{
						"spec": map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name": fmt.Sprintf("container-%s", containerIndex), // Placeholder name
										},
									},
								},
							},
						},
					}

					// Set the value at the property path
					if propertyPath != "" {
						// Remove the leading slash
						propertyPath = strings.TrimPrefix(propertyPath, "/")

						// Split the property path into parts
						parts := strings.Split(propertyPath, "/")

						// Navigate to the container in the merge patch
						container := mergePatch["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})

						// Build the nested structure for the property
						current := container
						for i := 0; i < len(parts)-1; i++ {
							part := parts[i]
							if _, ok := current[part]; !ok {
								current[part] = make(map[string]interface{})
							}
							current = current[part].(map[string]interface{})
						}

						// Set the value at the final part
						current[parts[len(parts)-1]] = addPatch["value"]
					}

					// Get the actual container name from the resource
					resource, err := p.dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
					if err != nil {
						fmt.Printf("Error getting resource: %v\n", err)
						return fmt.Errorf("error getting resource: %v", err)
					}

					// Extract the container name from the resource
					containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec", "containers")
					if err != nil || !found || len(containers) <= 0 {
						fmt.Printf("Error getting containers: %v\n", err)
						return fmt.Errorf("error getting containers: %v", err)
					}

					// Check if the container index is valid
					containerIdx, err := strconv.Atoi(containerIndex)
					if err != nil || containerIdx >= len(containers) {
						fmt.Printf("Invalid container index: %s\n", containerIndex)
						return fmt.Errorf("invalid container index: %s", containerIndex)
					}

					// Get the container name
					container, ok := containers[containerIdx].(map[string]interface{})
					if !ok {
						fmt.Printf("Container is not a map: %v\n", containers[containerIdx])
						return fmt.Errorf("container is not a map: %v", containers[containerIdx])
					}

					containerName, ok := container["name"].(string)
					if !ok {
						fmt.Printf("Container name is not a string: %v\n", container["name"])
						return fmt.Errorf("container name is not a string: %v", container["name"])
					}

					// Update the container name in the merge patch
					mergePatch["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})["name"] = containerName

					// Marshal the merge patch
					mergePatchJSON, err := json.Marshal(mergePatch)
					if err != nil {
						fmt.Printf("Error marshalling merge patch: %v\n", err)
						return fmt.Errorf("error marshalling merge patch: %v", err)
					}

					fmt.Printf("Applying container merge patch: %s\n", string(mergePatchJSON))

					// Apply the strategic merge patch
					_, err = p.dynamicClient.Resource(gvr).Namespace(namespace).Patch(
						context.TODO(),
						name,
						types.StrategicMergePatchType,
						mergePatchJSON,
						metav1.PatchOptions{},
					)

					if err != nil {
						fmt.Printf("Error applying container merge patch: %v\n", err)
						return fmt.Errorf("error applying container merge patch: %v", err)
					} else {
						fmt.Printf("Successfully applied container merge patch\n")
					}

					fmt.Printf("==== PatchK8sResource completed successfully ====\n\n")
					return nil
				}
			}

			if testOk && addOk &&
				(testPath == "/metadata/annotations" || testPath == "/metadata/labels") &&
				strings.HasPrefix(addPath, testPath+"/") {

				// This is a patch for annotations or labels
				fmt.Printf("Detected patch for %s, using strategic merge patch\n", testPath)

				// Extract the key from the add path
				key := strings.TrimPrefix(addPath, testPath+"/")
				// Unescape the key according to JSON Patch spec
				key = strings.ReplaceAll(key, "~1", "/")
				key = strings.ReplaceAll(key, "~0", "~")

				value := addPatch["value"]

				fmt.Printf("Key: %s, Value: %v\n", key, value)

				// Create a strategic merge patch
				var mergePatch map[string]interface{}

				if testPath == "/metadata/annotations" {
					mergePatch = map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": map[string]interface{}{
								key: value,
							},
						},
					}
				} else { // labels
					mergePatch = map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								key: value,
							},
						},
					}
				}

				mergePatchJSON, err := json.Marshal(mergePatch)
				if err != nil {
					fmt.Printf("Error marshalling merge patch: %v\n", err)
					return fmt.Errorf("error marshalling merge patch: %v", err)
				}

				fmt.Printf("Applying merge patch: %s\n", string(mergePatchJSON))

				_, err = p.dynamicClient.Resource(gvr).Namespace(namespace).Patch(
					context.TODO(),
					name,
					types.MergePatchType,
					mergePatchJSON,
					metav1.PatchOptions{},
				)

				if err != nil {
					fmt.Printf("Error applying merge patch: %v\n", err)
					return fmt.Errorf("error applying merge patch: %v", err)
				} else {
					fmt.Printf("Successfully applied merge patch\n")
				}

				fmt.Printf("==== PatchK8sResource completed successfully ====\n\n")
				return nil
			}
		}
	}

	// Apply each patch operation
	for i := 0; i < len(patches); i++ {
		patch := patches[i]
		patchMap, ok := patch.(map[string]interface{})
		if !ok {
			fmt.Printf("Patch %d is not a map, skipping\n", i)
			continue
		}

		op, _ := patchMap["op"].(string)
		path, _ := patchMap["path"].(string)

		fmt.Printf("Processing patch %d: op=%s, path=%s\n", i, op, path)

		// If this is a "test" operation followed by an "add" operation for a map key
		if op == "test" && i+1 < len(patches) {
			nextPatch, ok := patches[i+1].(map[string]interface{})
			if !ok {
				fmt.Printf("Next patch is not a map, skipping test handling\n")
				continue
			}
			nextOp, _ := nextPatch["op"].(string)
			nextPath, _ := nextPatch["path"].(string)

			fmt.Printf("Next patch: op=%s, path=%s\n", nextOp, nextPath)

			// If the next operation is "add" and the path is a child of the test path
			if nextOp == "add" && strings.HasPrefix(nextPath, path+"/") {
				fmt.Printf("Detected test+add pattern for map key\n")

				// Try to apply just the test patch to see if the map exists
				testPatchData, err := json.Marshal([]interface{}{patch})
				if err != nil {
					fmt.Printf("Error marshalling test patch: %v\n", err)
					return fmt.Errorf("error marshalling test patch: %v", err)
				}

				fmt.Printf("Applying test patch: %s\n", string(testPatchData))

				_, testErr := p.dynamicClient.Resource(gvr).Namespace(namespace).Patch(
					context.TODO(),
					name,
					types.JSONPatchType,
					testPatchData,
					metav1.PatchOptions{},
				)

				// If the test fails, the map doesn't exist, so we need to create it
				if testErr != nil {
					fmt.Printf("Test patch failed: %v\n", testErr)
					fmt.Printf("Creating map at path: %s\n", path)

					// Create a patch to add the empty map
					createMapPatch := []interface{}{
						map[string]interface{}{
							"op":    "add",
							"path":  path,
							"value": map[string]interface{}{},
						},
					}

					createMapPatchJSON, err := json.Marshal(createMapPatch)
					if err != nil {
						fmt.Printf("Error marshalling create map patch: %v\n", err)
						return fmt.Errorf("error marshalling create map patch: %v", err)
					}

					fmt.Printf("Applying create map patch: %s\n", string(createMapPatchJSON))

					// Apply the patch to create the map
					_, err = p.dynamicClient.Resource(gvr).Namespace(namespace).Patch(
						context.TODO(),
						name,
						types.JSONPatchType,
						createMapPatchJSON,
						metav1.PatchOptions{},
					)

					if err != nil {
						fmt.Printf("Error creating map at %s: %v\n", path, err)
						return fmt.Errorf("error creating map at %s: %v", path, err)
					} else {
						fmt.Printf("Successfully created map at %s\n", path)
					}

					// Skip the test patch and directly apply the add operation
					fmt.Printf("Skipping test patch, directly applying add patch\n")

					// Apply just the add operation
					addPatchData, err := json.Marshal([]interface{}{nextPatch})
					if err != nil {
						fmt.Printf("Error marshalling add patch: %v\n", err)
						return fmt.Errorf("error marshalling add patch: %v", err)
					}

					fmt.Printf("Applying add patch: %s\n", string(addPatchData))

					_, err = p.dynamicClient.Resource(gvr).Namespace(namespace).Patch(
						context.TODO(),
						name,
						types.JSONPatchType,
						addPatchData,
						metav1.PatchOptions{},
					)

					if err != nil {
						fmt.Printf("Error applying add patch: %v\n", err)
						return fmt.Errorf("error applying add patch: %v", err)
					} else {
						fmt.Printf("Successfully applied add patch\n")
					}

					// Skip both patches since we've handled them
					i++
					continue
				} else {
					fmt.Printf("Test patch succeeded, map exists\n")
				}
			}
		}

		// Apply the current patch
		patchData, err := json.Marshal([]interface{}{patch})
		if err != nil {
			fmt.Printf("Error marshalling patch: %v\n", err)
			return fmt.Errorf("error marshalling patch: %v", err)
		}

		fmt.Printf("Applying patch: %s\n", string(patchData))

		_, err = p.dynamicClient.Resource(gvr).Namespace(namespace).Patch(
			context.TODO(),
			name,
			types.JSONPatchType,
			patchData,
			metav1.PatchOptions{},
		)

		if err != nil {
			fmt.Printf("Error applying patch: %v\n", err)
			return fmt.Errorf("error applying patch: %v", err)
		} else {
			fmt.Printf("Successfully applied patch\n")
		}

		// Get state after patch
		_, err = p.dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Error getting updated state: %v\n", err)
		} else {
			fmt.Printf("Successfully retrieved updated state\n")
		}
	}

	fmt.Printf("==== PatchK8sResource completed successfully ====\n\n")
	return nil
}

type GroupVersion interface {
	Schema(contentType string) ([]byte, error)
}

func (p *APIServerProvider) GetOpenAPIResourceSpecs() (map[string][]string, error) {
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

		totalPaths := len(paths)

		// Create channels for work distribution and results collection
		type schemaResult struct {
			bytes []byte
			index int
		}

		// Create a slice to preserve order
		pathSlice := make([]GroupVersion, 0, len(paths))
		for _, path := range paths {
			pathSlice = append(pathSlice, path)
		}

		// Adjust to a more conservative number of workers
		numWorkers := runtime.NumCPU() // Just use number of CPUs instead of *2

		// Use smaller buffer sizes
		workChan := make(chan struct {
			path  GroupVersion
			index int
		}, len(pathSlice))
		resultChan := make(chan schemaResult, len(pathSlice))
		progressChan := make(chan int, len(pathSlice))

		// Feed the work channel
		for i, path := range pathSlice {
			workChan <- struct {
				path  GroupVersion
				index int
			}{path, i}
		}
		close(workChan)

		// Progress tracking goroutine
		go func() {
			processed := 0
			fmt.Print("\n🧠 Resolving schemas [")
			for range progressChan {
				processed++
				progress := (processed * 100) / len(pathSlice)
				fmt.Printf("\r🧠 Resolving schemas [%-25s] %d%%", strings.Repeat("=", progress/4), progress)
			}
			fmt.Print("\r")
		}()

		// Create worker pool
		var wg sync.WaitGroup
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for work := range workChan {
					time.Sleep(10 * time.Millisecond) // Add small delay between requests
					schemaBytes, err := work.path.Schema("application/com.github.proto-openapi.spec.v3@v1.0+protobuf")

					// Get the schema name from the path
					pathStr := fmt.Sprintf("%v", work.path)

					if err != nil {
						if !strings.Contains(err.Error(), "the backend attempted to redirect this request") {
							fmt.Printf("\nError getting schema %s: %v\n", pathStr, err)
						}
						progressChan <- 1
						continue
					}

					resultChan <- schemaResult{bytes: schemaBytes, index: work.index}
					progressChan <- 1
				}
			}()
		}

		// Start a goroutine to close channels when all workers are done
		go func() {
			wg.Wait()
			close(resultChan)
			close(progressChan)
		}()

		// Collect results in order
		schemasBytes := make([][]byte, totalPaths)
		p.openAPIDoc.Components.Schemas.AdditionalProperties = make([]*openapi_v3.NamedSchemaOrReference, 0, totalPaths)

		for result := range resultChan {
			schemasBytes[result.index] = result.bytes
		}

		// Process schemas in batches
		const batchSize = 10
		for i := 0; i < len(schemasBytes); i += batchSize {
			end := i + batchSize
			if end > len(schemasBytes) {
				end = len(schemasBytes)
			}

			var wg sync.WaitGroup
			for j := i; j < end; j++ {
				if len(schemasBytes[j]) == 0 {
					continue
				}
				wg.Add(1)
				go func(schemaBytes []byte) {
					defer wg.Done()
					tempDoc := &openapi_v3.Document{}
					if err := proto.Unmarshal(schemaBytes, tempDoc); err != nil {
						return
					}
					// Use a mutex when appending to shared slice
					if tempDoc.Components != nil && tempDoc.Components.Schemas != nil {
						p.openAPIDoc.Components.Schemas.AdditionalProperties = append(
							p.openAPIDoc.Components.Schemas.AdditionalProperties,
							tempDoc.Components.Schemas.AdditionalProperties...,
						)
					}
				}(schemasBytes[j])
			}
			wg.Wait()
		}
	}

	// Process schemas into field paths
	processed := 0
	specs := make(map[string][]string)
	if p.openAPIDoc.Components != nil && p.openAPIDoc.Components.Schemas != nil {
		// Create a cache for visited schemas to avoid reprocessing
		schemaCache := make(map[string][]string)

		for _, schemaEntry := range p.openAPIDoc.Components.Schemas.AdditionalProperties {
			resourceName := schemaEntry.Name
			schema := schemaEntry.Value.GetSchema()
			if schema != nil {
				// Check if we've already processed this schema
				if cachedFields, ok := schemaCache[resourceName]; ok {
					specs[resourceName] = cachedFields
					continue
				}

				visited := make(map[string][]string)
				fields := p.parseSchema(schema, "", visited, "")
				specs[resourceName] = fields

				// Cache the result
				schemaCache[resourceName] = fields
			}

			processed++
		}
	}
	fmt.Printf("\r ✔️ Resolving schemas (%v processed)                    \n", processed)

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
	// Create new config for the context
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: context,
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Get REST config for the context
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create config for context %s: %v", context, err)
	}

	// Create new clients for this context
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset for context %s: %v", context, err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client for context %s: %v", context, err)
	}

	// Create new provider with the context-specific clients
	return NewAPIServerProviderWithOptions(&APIServerProviderConfig{
		Clientset:     clientset,
		DynamicClient: dynamicClient,
		DryRun:        p.dryRun,
	})
}

// Add these methods to APIServerProvider

func (p *APIServerProvider) GetDiscoveryClient() (discovery.DiscoveryInterface, error) {
	return p.clientset.Discovery(), nil
}

func (p *APIServerProvider) GetClientset() (kubernetes.Interface, error) {
	return p.clientset, nil
}

func (p *APIServerProvider) GetGVRList() (map[string]schema.GroupVersionResource, error) {
	// Initialize the cache if it's empty
	if len(p.gvrCache) == 0 {
		err := p.initGVRCache()
		if err != nil {
			return nil, fmt.Errorf("error initializing GVR cache: %w", err)
		}
	}
	return p.GetGVRCacheSnapshot(), nil
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

			// Store fully qualified names
			if gv.Group != "" {
				// Store resource.group format
				p.gvrCache[r.Name+"."+gv.Group] = gvr
				if r.SingularName != "" {
					p.gvrCache[r.SingularName+"."+gv.Group] = gvr
				}
			}
		}
	}

	return nil
}

func (p *APIServerProvider) GetDynamicClient() (dynamic.Interface, error) {
	return p.dynamicClient, nil
}

// Add this method to APIServerProvider
func (p *APIServerProvider) GetGVRCacheSnapshot() map[string]schema.GroupVersionResource {
	p.gvrCacheMutex.RLock()
	defer p.gvrCacheMutex.RUnlock()

	// Return a copy of the cache to prevent concurrent access issues
	snapshot := make(map[string]schema.GroupVersionResource, len(p.gvrCache))
	for k, v := range p.gvrCache {
		snapshot[k] = v
	}
	return snapshot
}

// Add helper method to check if a resource is namespaced
func (p *APIServerProvider) isNamespacedResource(gvr schema.GroupVersionResource) (bool, error) {
	// Create cache key in format "group/version/resource"
	cacheKey := fmt.Sprintf("%s/%s/%s", gvr.Group, gvr.Version, gvr.Resource)

	// Check cache first
	p.namespacedMutex.RLock()
	if namespaced, exists := p.namespacedCache[cacheKey]; exists {
		p.namespacedMutex.RUnlock()
		return namespaced, nil
	}
	p.namespacedMutex.RUnlock()

	// If not in cache, fetch from API server
	resources, err := p.clientset.Discovery().ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		return false, fmt.Errorf("error getting server resources: %w", err)
	}

	// Update cache with the result
	p.namespacedMutex.Lock()
	defer p.namespacedMutex.Unlock()

	for _, r := range resources.APIResources {
		if r.Name == gvr.Resource {
			p.namespacedCache[cacheKey] = r.Namespaced
			return r.Namespaced, nil
		}
	}

	return false, fmt.Errorf("resource %q not found", gvr.Resource)
}
