package cmd

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// func extractJSON(data interface{}, path string) (string, error) {
// 	// Convert the data to JSON.
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return "", err
// 	}

// 	// Parse the JSONPath expression.
// 	expr, err := jsonpath.Read(jsonData, path)
// 	if err != nil {
// 		return "", err
// 	}

// 	// Convert the result to a string.
// 	result, ok := expr.(string)
// 	if !ok {
// 		return "", fmt.Errorf("expected string result, got %T", expr)
// 	}

// 	// Trim any leading/trailing whitespace and return the result.
// 	return strings.TrimSpace(result), nil
// }

type QueryExecutor struct {
	Clientset *kubernetes.Clientset
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

	return &QueryExecutor{Clientset: clientset}, nil
}

func getK8sResources(clientset *kubernetes.Clientset, kind string) (interface{}, error) {
	switch kind {
	case "Pod":
		// Get the list of pods using the k8s client's corev1 method.
		pods, err := clientset.CoreV1().Pods(Namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Println("Error getting list of pods: ", err)
			return nil, err
		}
		return pods, nil
	case "Deployment":
		// Get the list of deployments using the k8s client's appsv1 method.
		deployments, err := clientset.AppsV1().Deployments(Namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Println("Error getting list of deployments: ", err)
			return nil, err
		}
		return deployments, nil
	case "Service":
		// Get the list of services using the k8s client's corev1 method.
		services, err := clientset.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Println("Error getting list of services: ", err)
			return nil, err
		}
		return services, nil
	case "ConfigMap":
		// Get the list of configmaps using the k8s client's corev1 method.
		configmaps, err := clientset.CoreV1().ConfigMaps("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Println("Error getting list of configmaps: ", err)
			return nil, err
		}
		return configmaps, nil
	case "Secret":
		// Get the list of secrets using the k8s client's corev1 method.
		secrets, err := clientset.CoreV1().Secrets("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Println("Error getting list of secrets: ", err)
			return nil, err
		}
		return secrets, nil
	default:
		return nil, fmt.Errorf("unknown kind: %s", kind)
	}
}

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
			list, err := getK8sResources(q.Clientset, kind)
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
			// jsonPath := c.JsonPath
			// // Extract the JSONPath from the results.
			// result, err := extractJSON(results, "$"+jsonPath)
			// if err != nil {
			// 	fmt.Println("Error extracting JSONPath: ", err)
			// 	return nil, err
			// }
			return results, nil
		default:
			return nil, fmt.Errorf("unknown clause type: %T", c)
		}
	}

	// After executing all clauses, format the results according to the ReturnClause.
	// ...

	return results, nil
}

// Implement specific methods to handle each clause type.
