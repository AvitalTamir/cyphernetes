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

// Rest of the code remains the same...

func (q *QueryExecutor) Execute(ast *Expression) (interface{}, error) {
	// Initialize the results variable.
	var results interface{}
	var resultMap map[string]interface{}
	var list unstructured.UnstructuredList
	var resultMapJson []byte
	var k8sResources interface{}

	// Iterate over the clauses in the AST.
	for _, clause := range ast.Clauses {
		switch c := clause.(type) {
		case *MatchClause:
			debugLog("Executing Kubernetes list operation for Name:", c.NodePattern.Name, "Kind:", c.NodePattern.Kind)
			name, kind := c.NodePattern.Name, c.NodePattern.Kind

			if c.NodePattern.Properties != nil && len(c.NodePattern.Properties.PropertyList) > 0 {
				for i, prop := range c.NodePattern.Properties.PropertyList {
					if prop.Key == "namespace" || prop.Key == "metadata.namespace" {
						Namespace = prop.Value.(string)
						// Remove the namespace slice from the properties
						c.NodePattern.Properties.PropertyList = append(c.NodePattern.Properties.PropertyList[:i], c.NodePattern.Properties.PropertyList[i+1:]...)
						fmt.Println("Namespace:", Namespace)
					}
				}
			}

			var fieldSelector string
			var labelSelector string
			var hasNameSelector bool
			if c.NodePattern.Properties != nil {
				for _, prop := range c.NodePattern.Properties.PropertyList {
					if prop.Key == "name" || prop.Key == "metadata.name" {
						fieldSelector += fmt.Sprintf("metadata.name=%s,", prop.Value)
						hasNameSelector = true
					} else {
						if hasNameSelector {
							// both name and label selectors are specified, error out
							return nil, fmt.Errorf("the 'name' selector can be used by itself or combined with 'namespace', but not with other label selectors")
						}
						labelSelector += fmt.Sprintf("%s=%s,", prop.Key, prop.Value)
					}
				}
				fieldSelector = strings.TrimSuffix(fieldSelector, ",")
				labelSelector = strings.TrimSuffix(labelSelector, ",")

			}

			// Get the list of resources of the specified kind.
			var err error
			list, err = q.getK8sResources(kind, fieldSelector, labelSelector)
			if err != nil {
				fmt.Println("Error getting list of resources: ", err)
				return nil, err
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
			resultMap[name] = converted
			resultMapJson, err = json.Marshal(resultMap)
			if err != nil {
				fmt.Println("Error marshalling results to JSON: ", err)
				return nil, err
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

			// Make sure query starts with '$'
			if !strings.HasPrefix(c.JsonPath, "$") {
				c.JsonPath = "$." + c.JsonPath
			}

			// if there's a nil key in jsonData, convert it to empty array
			jsonData = convertNilKey(jsonData)
			results, err := jsonpath.JsonPathLookup(jsonData, c.JsonPath)
			if err != nil {
				fmt.Println("Path not found:", c.JsonPath)
				return nil, err
			}
			k8sResources = results

		default:
			return nil, fmt.Errorf("unknown clause type: %T", c)
		}
	}

	// After executing all clauses, format the results according to the ReturnClause.
	// ...

	return k8sResources, nil
}

func convertNilKey(jsonData interface{}) interface{} {
	switch jsonData.(type) {
	case map[string]interface{}:
		for k, v := range jsonData.(map[string]interface{}) {
			if v == nil {
				jsonData.(map[string]interface{})[k] = []interface{}{}
			} else {
				jsonData.(map[string]interface{})[k] = convertNilKey(v)
			}
		}
	case []interface{}:
		for i, v := range jsonData.([]interface{}) {
			jsonData.([]interface{})[i] = convertNilKey(v)
		}
	}
	return jsonData
}
