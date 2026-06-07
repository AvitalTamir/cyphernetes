package core

import "fmt"

func (q *QueryExecutor) buildGraph(result *QueryResult) {
	debugLog("Building graph")
	debugLog("Initial nodes: %+v\n", result.Graph.Nodes)

	nodes := []Node{}
	nodeMap := make(map[string]bool)
	for _, node := range result.Graph.Nodes {
		addGraphNode(&nodes, nodeMap, node)
	}

	// Process nodes from result data
	for key, resources := range result.Data {
		resourcesSlice, ok := resources.([]interface{})
		if !ok || len(resourcesSlice) == 0 {
			continue
		}
		for _, resource := range resourcesSlice {
			resourceMap, ok := resource.(map[string]interface{})
			if !ok {
				continue
			}
			metadata, err := getResourceMetadata(resourceMap)
			if err != nil {
				continue
			}
			name, err := getResourceName(metadata)
			if err != nil {
				continue
			}
			kind, err := getResourceKind(resourceMap)
			if err != nil {
				continue
			}
			node := Node{
				Id:   key,
				Kind: kind,
				Name: name,
			}
			if node.Kind != "Namespace" {
				node.Namespace = getNamespaceName(metadata)
			}
			debugLog("Adding node from result data: %+v\n", node)
			addGraphNode(&nodes, nodeMap, node)
		}
	}
	result.Graph.Nodes = nodes

	// Process edges
	var edges []Edge
	edgeMap := make(map[string]bool)
	for _, edge := range result.Graph.Edges {
		edgeKey := fmt.Sprintf("%s-%s-%s", edge.From, edge.To, edge.Type)
		reverseEdgeKey := fmt.Sprintf("%s-%s-%s", edge.To, edge.From, edge.Type)

		if !edgeMap[edgeKey] && !edgeMap[reverseEdgeKey] {
			edges = append(edges, edge)
			edgeMap[edgeKey] = true
			edgeMap[reverseEdgeKey] = true
		}
	}
	result.Graph.Edges = edges
}

func addGraphNode(nodes *[]Node, seen map[string]bool, node Node) {
	key := fmt.Sprintf("%s/%s/%s", node.Kind, node.Namespace, node.Name)
	if !seen[key] {
		seen[key] = true
		*nodes = append(*nodes, node)
	}
}

func getNamespaceName(metadata map[string]interface{}) string {
	namespace, ok := metadata["namespace"].(string)
	if !ok {
		namespace = "default"
	}
	return namespace
}

func getTargetK8sResourceName(resourceTemplate map[string]interface{}, resourceName string, foreignName string) string {
	// We'll use these in order of preference:
	// 1. The .name or .metadata.name specified in the resource template
	// 2. The name of the kubernetes resource represented by the foreign node
	// 3. The name of the node
	name := ""
	if foreignName != "" {
		name = foreignName
	} else if resourceTemplate["name"] != nil {
		name = resourceTemplate["name"].(string)
	} else if resourceTemplate["metadata"] != nil && resourceTemplate["metadata"].(map[string]interface{})["name"] != nil {
		name = resourceTemplate["metadata"].(map[string]interface{})["name"].(string)
	} else {
		name = resourceName
	}
	return name
}
