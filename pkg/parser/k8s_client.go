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
	"k8s.io/client-go/tools/clientcmd"
)

var (
	executorInstance *QueryExecutor
	once             sync.Once
)

func GetQueryExecutorInstance() *QueryExecutor {
	once.Do(func() {
		executor, err := NewQueryExecutor()
		if err != nil {
			// Handle error
			fmt.Println("Error creating QueryExecutor instance:", err)
			return
		}
		executorInstance = executor
	})
	return executorInstance
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
	responseChan  chan *apiResponse
}

type apiResponse struct {
	list *unstructured.UnstructuredList
	err  error
}

func NewQueryExecutor() (*QueryExecutor, error) {
	// Use the local kubeconfig context
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		fmt.Println("Error creating in-cluster config")
		return nil, err
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("Error creating clientset")
		return nil, err
	}

	// Create the dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Println("Error creating dynamic client")
		return nil, err
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

func (q *QueryExecutor) processRequests() {
	for request := range q.requestChannel {
		q.semaphore <- struct{}{} // Acquire a token
		list, err := q.fetchResources(request.kind, request.fieldSelector, request.labelSelector)
		<-q.semaphore // Release the token
		request.responseChan <- &apiResponse{list: &list, err: err}
	}
}

func (q *QueryExecutor) getK8sResources(kind string, fieldSelector string, labelSelector string) (*unstructured.UnstructuredList, error) {
	responseChan := make(chan *apiResponse)
	q.requestChannel <- &apiRequest{
		kind:          kind,
		fieldSelector: fieldSelector,
		labelSelector: labelSelector,
		responseChan:  responseChan,
	}

	response := <-responseChan
	return response.list, response.err
}

func (q *QueryExecutor) fetchResources(kind string, fieldSelector string, labelSelector string) (unstructured.UnstructuredList, error) {
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

	if AllNamespaces {
		Namespace = ""
	}
	list, err := q.DynamicClient.Resource(gvr).Namespace(Namespace).List(context.Background(), metav1.ListOptions{
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
	discoveryClient := clientset.Discovery()
	if apiResourceListCache == nil {
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
