package cmd

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type QueryExecutor struct {
	Clientset      *kubernetes.Clientset
	DynamicClient  dynamic.Interface
	requestChannel chan *apiRequest
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

	executor := &QueryExecutor{
		Clientset:      clientset,
		DynamicClient:  dynamicClient,
		requestChannel: make(chan *apiRequest, 1), // Buffer size can be adjusted
	}

	go executor.processRequests()

	return executor, nil
}

func (q *QueryExecutor) processRequests() {
	for request := range q.requestChannel {
		list, err := q.fetchResources(request.kind, request.fieldSelector, request.labelSelector)
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
	gvr, err := findGVR(q.Clientset, kind)
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

	if allNamespaces {
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

func findGVR(clientset *kubernetes.Clientset, resourceIdentifier string) (schema.GroupVersionResource, error) {
	discoveryClient := clientset.Discovery()

	// Get the list of API resources
	apiResourceList, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	// Normalize the resource identifier to lower case for case-insensitive comparison
	normalizedIdentifier := strings.ToLower(resourceIdentifier)

	for _, apiResource := range apiResourceList {
		for _, resource := range apiResource.APIResources {
			// Check if the resource name, kind, or short names match the specified identifier
			if strings.EqualFold(resource.Name, normalizedIdentifier) || // Plural name match
				strings.EqualFold(resource.Kind, resourceIdentifier) || // Kind name match
				containsIgnoreCase(resource.ShortNames, normalizedIdentifier) { // Short name match

				gv, err := schema.ParseGroupVersion(apiResource.GroupVersion)
				if err != nil {
					return schema.GroupVersionResource{}, err
				}
				return gv.WithResource(resource.Name), nil
			}
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("resource identifier not found: %s", resourceIdentifier)
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
