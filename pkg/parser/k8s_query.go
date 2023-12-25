package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/oliveagle/jsonpath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var resultCache = make(map[string]interface{})
var resultMap = make(map[string]interface{})

func (q *QueryExecutor) Execute(ast *Expression) (interface{}, error) {
	k8sResources := make(map[string]interface{})

	// Iterate over the clauses in the AST.
	for _, clause := range ast.Clauses {
		switch c := clause.(type) {
		case *MatchClause:
			// we'll begin by fetching our relationships which mean fetching all kubernetes resources selectable by relationships in our clause
			// we'll build a map of the resources we find, keyed by the name of the resource
			// After finishing with relationships, we'll move on to nodes and add them to the map
			// throughout the process we'll an intermediary struct between kubernetes and the map as cache, it will hold the complete structs from k8s to avoid fetching the same resource twice
			// when iterating over nodes, no node will be refetched that has already been fetched in the relationship phase,
			// important: during the relationships phase, before fetching a resource from kubernetes note that our relationships hold only ResourceProperties.Name and ResourceProperties.Kind, so must refer to the matching node in our nodes to get the full selector

			// Iterate over the relationships in the match clause.
			// Process Relationships
			for _, rel := range c.Relationships {
				// Determine relationship type and fetch related resources
				var relType RelationshipType
				leftKind, err := FindGVR(q.Clientset, rel.LeftNode.ResourceProperties.Kind)
				if err != nil {
					fmt.Println("Error finding API resource: ", err)
					return nil, err
				}
				rightKind, err := FindGVR(q.Clientset, rel.RightNode.ResourceProperties.Kind)
				if err != nil {
					fmt.Println("Error finding API resource: ", err)
					return nil, err
				}

				for _, resourceRelationship := range relationshipRules {
					if (strings.EqualFold(leftKind.Resource, resourceRelationship.KindA) && strings.EqualFold(rightKind.Resource, resourceRelationship.KindB)) ||
						(strings.EqualFold(rightKind.Resource, resourceRelationship.KindA) && strings.EqualFold(leftKind.Resource, resourceRelationship.KindB)) {
						relType = resourceRelationship.Relationship
					}
				}

				if relType == "" {
					// no relationship type found, error out
					return nil, fmt.Errorf("relationship type not found between %s and %s", leftKind, rightKind)
				}

				rule := findRuleByRelationshipType(relType)
				if err != nil {
					fmt.Println("Error determining relationship type: ", err)
					return nil, err
				}

				// Fetch and process related resources based on relationship type
				for _, node := range c.Nodes {
					if node.ResourceProperties.Name == rel.LeftNode.ResourceProperties.Name || node.ResourceProperties.Name == rel.RightNode.ResourceProperties.Name {
						if resultMap[node.ResourceProperties.Name] == nil {
							getNodeResources(node, q)
						}

					}
				}
				var resourcesAInterface interface{}
				var resourcesBInterface interface{}
				var filteredDirection Direction

				if rule.KindA == rightKind.Resource {
					resourcesAInterface = resultMap[rel.RightNode.ResourceProperties.Name]
					resourcesBInterface = resultMap[rel.LeftNode.ResourceProperties.Name]
					filteredDirection = Left
				} else if rule.KindA == leftKind.Resource {
					resourcesAInterface = resultMap[rel.LeftNode.ResourceProperties.Name]
					resourcesBInterface = resultMap[rel.RightNode.ResourceProperties.Name]
					filteredDirection = Right
				} else {
					// error out
					return nil, fmt.Errorf("relationship rule not found for %s and %s - This code path should be invalid, likely problem with rule definitions", rel.LeftNode.ResourceProperties.Kind, rel.RightNode.ResourceProperties.Kind)
				}
				// Apply relationship rules to filter resources
				resourcesA, okA := resourcesAInterface.([]map[string]interface{})
				resourcesB, okB := resourcesBInterface.([]map[string]interface{})

				if !okA || !okB {
					fmt.Println("Type assertion failed for resources")
					continue
				}

				matchedResources := applyRelationshipRule(resourcesA, resourcesB, rule, filteredDirection)
				resultMap[rel.RightNode.ResourceProperties.Name] = matchedResources
			}

			// Iterate over the nodes in the match clause.
			for _, node := range c.Nodes {
				debugLog("Node pattern found. Name:", node.ResourceProperties.Name, "Kind:", node.ResourceProperties.Kind)
				// check if the node has already been fetched
				if resultCache[q.resourcePropertyName(node)] == nil {
					getNodeResources(node, q)
				} else if resultMap[node.ResourceProperties.Name] == nil {
					resultMap[node.ResourceProperties.Name] = resultCache[q.resourcePropertyName(node)]
				}
			}
			// case *CreateClause:
			// 	// Execute a Kubernetes create operation based on the CreateClause.
			// 	// ...
		case *SetClause:
			// Execute a Kubernetes update operation based on the SetClause.
			// ...
			for idx, kvp := range c.KeyValuePairs {
				resultMapKey := strings.Split(kvp.Key, ".")[0]
				path := strings.Split(kvp.Key, ".")[1:]

				patch := make(map[string]interface{})
				// Drill down to create nested map structure
				for i, part := range path {
					if i == len(path)-1 {
						// Last part: assign the result
						patch[part] = kvp.Value
					} else {
						// Intermediate parts: create nested maps
						if patch[part] == nil {
							patch[part] = make(map[string]interface{})
						}
						patch = patch[part].(map[string]interface{})
					}
				}

				// Patch should be in this format: [{"op": "replace", "path": "/spec/replicas", "value": $value}],
				// this means we need to join the path parts with '/' and add the value
				pathStr := "/" + strings.Join(path, "/")
				patchJson, err := json.Marshal([]map[string]interface{}{{"op": "replace", "path": pathStr, "value": kvp.Value}})
				if err != nil {
					fmt.Println("Error marshalling patch to JSON: ", err)
					return nil, err
				}

				// Apply the patch to the resources
				err = q.patchK8sResources(resultMapKey, patchJson)
				if err != nil {
					fmt.Println("Error patching resource: ", err)
					return nil, err
				}

				// Retrieve the slice of maps for the resultMapKey
				if resources, ok := resultMap[resultMapKey].([]map[string]interface{}); ok {
					// Check if the idx is within bounds
					if idx >= 0 && idx < len(resources) {
						entry := resources[idx]             // Get the specific map to patch
						fullPath := strings.Join(path, ".") // Construct the full path
						patchResultMap(entry, fullPath, kvp.Value)
						resources[idx] = entry // Update the specific entry in the slice
					} else {
						fmt.Printf("Index out of range for key: %s, Index: %d, Length: %d\n", resultMapKey, idx, len(resources))
						// Handle index out of range
					}
				} else {
					// Handle the case where the resultMap entry isn't a slice of maps
					fmt.Printf("Failed to assert type for key: %s, Expected: []map[string]interface{}, Actual: %T\n", resultMapKey, resultMap[resultMapKey])
					// You may want to handle this case according to your application's needs
				}

			}

		case *DeleteClause:
			// Execute a Kubernetes delete operation based on the DeleteClause.
			for _, nodeId := range c.NodeIds {
				// make sure the identifier is a key in the result map
				if resultMap[nodeId] == nil {
					return nil, fmt.Errorf("node identifier %s not found in result map", nodeId)
				}
				q.deleteK8sResources(nodeId)
			}

		case *ReturnClause:
			resultMapJson, err := json.Marshal(resultMap)
			if err != nil {
				fmt.Println("Error marshalling results to JSON: ", err)
				return nil, err
			}
			var jsonData interface{}
			json.Unmarshal(resultMapJson, &jsonData)

			for _, jsonPath := range c.JsonPaths {
				// Ensure the JSONPath starts with '$'
				if !strings.HasPrefix(jsonPath, "$") {
					jsonPath = "$." + jsonPath
				}

				pathParts := strings.Split(jsonPath, ".")[1:]

				// Drill down to create nested map structure
				currentMap := k8sResources
				for i, part := range pathParts {
					if i == len(pathParts)-1 {
						// Last part: assign the result
						result, err := jsonpath.JsonPathLookup(jsonData, jsonPath)
						if err != nil {
							logDebug("Path not found:", jsonPath)
							result = []interface{}{}
						}
						currentMap[part] = result
					} else {
						// Intermediate parts: create nested maps
						if currentMap[part] == nil {
							currentMap[part] = make(map[string]interface{})
						}
						currentMap = currentMap[part].(map[string]interface{})
					}
				}
			}

		default:
			return nil, fmt.Errorf("unknown clause type: %T", c)
		}
	}
	// clear the result cache and result map
	resultCache = make(map[string]interface{})
	resultMap = make(map[string]interface{})
	return k8sResources, nil
}

func (q *QueryExecutor) deleteK8sResources(nodeId string) error {
	resources := resultMap[nodeId].([]map[string]interface{})

	for i := range resources {
		// Look up the resource kind and name in the cache
		gvr, err := FindGVR(q.Clientset, resources[i]["kind"].(string))
		if err != nil {
			fmt.Printf("Error finding API resource: %v\n", err)
			return err
		}
		resourceName := resultMap[nodeId].([]map[string]interface{})[i]["metadata"].(map[string]interface{})["name"].(string)

		err = q.DynamicClient.Resource(gvr).Namespace(Namespace).Delete(context.Background(), resourceName, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("Error deleting resource: %v\n", err)
			return err
		}

		// remove the resource from the result map
		delete(resultMap, nodeId)
	}

	return nil
}

func (q *QueryExecutor) patchK8sResources(resultMapKey string, patch []byte) error {
	resources := resultMap[resultMapKey].([]map[string]interface{})

	for i := range resources {
		// Look up the resource kind and name in the cache
		gvr, err := FindGVR(q.Clientset, resources[i]["kind"].(string))
		if err != nil {
			fmt.Printf("Error finding API resource: %v\n", err)
			return err
		}
		resourceName := resultMap[resultMapKey].([]map[string]interface{})[i]["metadata"].(map[string]interface{})["name"].(string)

		_, err = q.DynamicClient.Resource(gvr).Namespace(Namespace).Patch(context.Background(), resourceName, types.JSONPatchType, patch, metav1.PatchOptions{})
		if err != nil {
			fmt.Printf("Error patching resource: %v\n", err)
			return err
		}

		// refresh the resource in the cache

	}
	return nil
}

func getNodeResources(n *NodePattern, q *QueryExecutor) (err error) {
	if n.ResourceProperties.Properties != nil && len(n.ResourceProperties.Properties.PropertyList) > 0 {
		for i, prop := range n.ResourceProperties.Properties.PropertyList {
			if prop.Key == "namespace" || prop.Key == "metadata.namespace" {
				Namespace = prop.Value.(string)
				// Remove the namespace slice from the properties
				n.ResourceProperties.Properties.PropertyList = append(n.ResourceProperties.Properties.PropertyList[:i], n.ResourceProperties.Properties.PropertyList[i+1:]...)
			}
		}
	}

	var fieldSelector string
	var labelSelector string
	var hasNameSelector bool
	if n.ResourceProperties.Properties != nil {
		for _, prop := range n.ResourceProperties.Properties.PropertyList {
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
	// Check if the resource has already been fetched
	if resultCache[q.resourcePropertyName(n)] == nil {
		// Get the list of resources of the specified kind.
		resultCache[q.resourcePropertyName(n)], err = q.getResources(n.ResourceProperties.Kind, fieldSelector, labelSelector)
		if err != nil {
			fmt.Println("Error marshalling results to JSON: ", err)
			return err
		}
	} else {
		fmt.Println("Resource already fetched")
	}
	resultMap[n.ResourceProperties.Name] = resultCache[q.resourcePropertyName(n)]
	return nil
}

func (q *QueryExecutor) getResources(kind, fieldSelector, labelSelector string) (interface{}, error) {
	list, err := q.getK8sResources(kind, fieldSelector, labelSelector)
	if err != nil {
		fmt.Println("Error getting list of resources: ", err)
		return nil, err
	}
	// merge list into resultCache
	var converted []map[string]interface{}
	for _, u := range list.Items {
		converted = append(converted, u.UnstructuredContent())
	}
	return converted, nil
}

func (q *QueryExecutor) resourcePropertyName(n *NodePattern) string {
	var ns string

	gvr, err := FindGVR(q.Clientset, n.ResourceProperties.Kind)
	if err != nil {
		fmt.Println("Error finding API resource: ", err)
		return ""
	}

	if n.ResourceProperties.Properties == nil {
		return fmt.Sprintf("%s_%s", Namespace, gvr.Resource)
	}
	for _, prop := range n.ResourceProperties.Properties.PropertyList {
		if prop.Key == "namespace" || prop.Key == "metadata.namespace" {
			ns = fmt.Sprint(prop.Value)
		}
	}
	if ns == "" {
		ns = Namespace
	}

	var keyValuePairs []string
	for _, prop := range n.ResourceProperties.Properties.PropertyList {
		// Convert the value to a string regardless of its actual type
		valueStr := fmt.Sprint(prop.Value)
		keyValuePairs = append(keyValuePairs, prop.Key+"_"+valueStr)
	}

	// Sort the key-value pairs to ensure consistency
	sort.Strings(keyValuePairs)

	// Join all key-value pairs with "_"
	joinedPairs := strings.Join(keyValuePairs, "_")

	// Return the formatted string
	return fmt.Sprintf("%s_%s_%s", ns, gvr.Resource, joinedPairs)
}

func patchResultMap(result map[string]interface{}, fullPath string, newValue interface{}) {
	parts := strings.Split(fullPath, ".") // Split the path into parts

	if len(parts) == 1 {
		// If we're at the end of the path, set the value directly
		result[parts[0]] = newValue
		return
	}

	// If we're not at the end, move down one level in the path
	nextLevel := parts[0]
	remainingPath := strings.Join(parts[1:], ".")

	if nextMap, ok := result[nextLevel].(map[string]interface{}); ok {
		// If the next level is a map, continue patching
		patchResultMap(nextMap, remainingPath, newValue)
	} else {
		// If the next level is not a map, it needs to be created
		newMap := make(map[string]interface{})
		result[nextLevel] = newMap
		patchResultMap(newMap, remainingPath, newValue)
	}
}
