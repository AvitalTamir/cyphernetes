package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oliveagle/jsonpath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type QueryExecutor struct {
	Clientset     *kubernetes.Clientset
	DynamicClient dynamic.Interface
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

	return &QueryExecutor{Clientset: clientset, DynamicClient: dynamicClient}, nil
}

func (q *QueryExecutor) getK8sResources(kind string, fieldSelector string, labelSelector string) (unstructured.UnstructuredList, error) {
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

// Initialize the results variable.
var results interface{}
var resultMap map[string]interface{}
var resultMapJson []byte

func (q *QueryExecutor) Execute(ast *Expression) (interface{}, error) {
	k8sResources := make(map[string]interface{})

	// Iterate over the clauses in the AST.
	for _, clause := range ast.Clauses {
		switch c := clause.(type) {
		case *MatchClause:
			debugLog("Node pattern found. Name:", c.NodePattern.Name, "Kind:", c.NodePattern.Kind)
			getNodeResouces(c.NodePattern, q)
			if c.ConnectedNodePattern != nil {
				debugLog("Node pattern found. Name:", c.ConnectedNodePattern.Name, "Kind:", c.ConnectedNodePattern.Kind)
				getNodeResouces(c.ConnectedNodePattern, q)
			}

			// case *CreateClause:
			// 	// Execute a Kubernetes create operation based on the CreateClause.
			// 	// ...
			// case *SetClause:
			// 	// Execute a Kubernetes update operation based on the SetClause.
			// 	// ...
			// case *DeleteClause:
			// 	// Execute a Kubernetes delete operation based on the DeleteClause.
			// 	// ...
		case *ReturnClause:
			var jsonData interface{}
			json.Unmarshal(resultMapJson, &jsonData)

			for _, jsonPath := range c.JsonPaths {
				// Ensure the JSONPath starts with '$'
				if !strings.HasPrefix(jsonPath, "$") {
					jsonPath = "$." + jsonPath
				}

				// Grab the base name of the node pattern from the JSONPath (the part between $. and the first [., [ or space)
				baseName := strings.Split(jsonPath, ".")[1]
				baseName = strings.Split(baseName, "[")[0]

				// Convert nil keys in jsonData to empty array if necessary
				jsonData = convertNilKey(jsonData)

				result, err := jsonpath.JsonPathLookup(jsonData, jsonPath)
				if err != nil {
					logDebug("Path not found:", jsonPath)
					// result gets empty array if path not found
					result = []interface{}{}
				}

				k8sResources[baseName] = result
			}

		default:
			return nil, fmt.Errorf("unknown clause type: %T", c)
		}
	}

	// After executing all clauses, format the results according to the ReturnClause.
	// ...

	return k8sResources, nil
}

func convertNilKey(jsonData interface{}) interface{} {
	switch jsonData := jsonData.(type) {
	case map[string]interface{}:
		for k, v := range jsonData {
			jsonData[k] = convertNilKey(v)
		}
	case []interface{}:
		for i, v := range jsonData {
			jsonData[i] = convertNilKey(v)
		}
	case nil:
		return []interface{}{}
	}
	return jsonData
}

func getNodeResouces(n *NodePattern, q *QueryExecutor) (err error) {
	if n.Properties != nil && len(n.Properties.PropertyList) > 0 {
		for i, prop := range n.Properties.PropertyList {
			if prop.Key == "namespace" || prop.Key == "metadata.namespace" {
				Namespace = prop.Value.(string)
				// Remove the namespace slice from the properties
				n.Properties.PropertyList = append(n.Properties.PropertyList[:i], n.Properties.PropertyList[i+1:]...)
			}
		}
	}

	var fieldSelector string
	var labelSelector string
	var hasNameSelector bool
	if n.Properties != nil {
		for _, prop := range n.Properties.PropertyList {
			if prop.Key == "name" || prop.Key == "metadata.name" {
				fieldSelector += fmt.Sprintf("metadata.name=%s,", prop.Value)
				hasNameSelector = true
			} else {
				if hasNameSelector {
					// both name and label selectors are specified, error out
					return fmt.Errorf("the 'name' selector can be used by itself or combined with 'namespace', but not with other label selectors")
				}
				labelSelector += fmt.Sprintf("%s=%s,", prop.Key, prop.Value)
			}
		}
		fieldSelector = strings.TrimSuffix(fieldSelector, ",")
		labelSelector = strings.TrimSuffix(labelSelector, ",")

	}

	// Get the list of resources of the specified kind.
	list, err := q.getK8sResources(n.Kind, fieldSelector, labelSelector)
	if err != nil {
		fmt.Println("Error getting list of resources: ", err)
		return err
	}

	var converted []map[string]interface{}
	for _, u := range list.Items {
		converted = append(converted, u.UnstructuredContent())
	}
	// Initialize results as a map if not already done
	if results == nil {
		results = make(map[string]interface{})
	}

	// Add the list to the results under the 'name' key
	resultMap = results.(map[string]interface{})
	resultMap[n.Name] = converted
	resultMapJson, err = json.Marshal(resultMap)
	if err != nil {
		fmt.Println("Error marshalling results to JSON: ", err)
		return err
	}
	return nil
}
