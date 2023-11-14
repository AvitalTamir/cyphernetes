package cmd

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (q *QueryExecutor) getK8sResources(kind string) (interface{}, error) {
	// Use discovery client to find the GVR for the given kind
	gvr, err := findGVR(q.Clientset, kind)
	if err != nil {
		return nil, err
	}

	// Use dynamic client to list resources
	list, err := q.DynamicClient.Resource(gvr).Namespace(Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Println("Error getting list of resources: ", err)
		return nil, err
	}
	return list, nil
}

func findGVR(clientset *kubernetes.Clientset, kind string) (schema.GroupVersionResource, error) {
	discoveryClient := clientset.Discovery()

	// Get the list of API resources
	apiResourceList, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	for _, apiResource := range apiResourceList {
		for _, resource := range apiResource.APIResources {
			// Check if the resource Kind matches the specified kind
			if strings.EqualFold(resource.Kind, kind) {
				gv, err := schema.ParseGroupVersion(apiResource.GroupVersion)
				if err != nil {
					return schema.GroupVersionResource{}, err
				}
				return gv.WithResource(resource.Name), nil
			}
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("resource kind not found: %s", kind)
}

// Rest of the code remains the same...

func (q *QueryExecutor) Execute(ast *Expression) (interface{}, error) {
	// Initialize the results variable.
	var results interface{}
	// Iterate over the clauses in the AST.
	for _, clause := range ast.Clauses {
		switch c := clause.(type) {
		case *MatchClause:
			// Execute a Kubernetes list operation based on the MatchClause.
			// ...
			fmt.Println("Executing Kubernetes list operation for Name:", c.NodePattern.Name, "Kind:", c.NodePattern.Kind)
			name, kind := c.NodePattern.Name, c.NodePattern.Kind
			// Get the list of resources of the specified kind using the k8s client's custom resource method.
			list, err := q.getK8sResources(kind)
			if err != nil {
				fmt.Println("Error getting list of resources: ", err)
				return nil, err
			}
			results = make(map[string]interface{})
			results.(map[string]interface{})[name] = list
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
			// Format the results according to the ReturnClause.
			// ...
			// Extract the JSONPath from the ReturnClause.
			jsonPath := c.JsonPath
			// // TODO: Extract the JSONPath from the results.
			// result, err := extractJSON(results, "$"+jsonPath)
			// if err != nil {
			// 	fmt.Println("Error extracting JSONPath: ", err)
			// 	return nil, err
			// }
			return results.(map[string]interface{})[jsonPath], nil
		default:
			return nil, fmt.Errorf("unknown clause type: %T", c)
		}
	}

	// After executing all clauses, format the results according to the ReturnClause.
	// ...

	return results, nil
}

// Implement specific methods to handle each clause type.
