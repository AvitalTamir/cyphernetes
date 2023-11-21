package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/oliveagle/jsonpath"
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
				leftKind, err := findGVR(q.Clientset, rel.LeftNode.ResourceProperties.Kind)
				if err != nil {
					fmt.Println("Error finding API resource: ", err)
					return nil, err
				}
				rightKind, err := findGVR(q.Clientset, rel.RightNode.ResourceProperties.Kind)
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
							getNodeResouces(node, q)
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
				if resultCache[resourcePropertyName(node)] == nil {
					getNodeResouces(node, q)
				} else if resultMap[node.ResourceProperties.Name] == nil {
					resultMap[node.ResourceProperties.Name] = resultCache[resourcePropertyName(node)]
				}
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

func getNodeResouces(n *NodePattern, q *QueryExecutor) (err error) {
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
	if resultCache[resourcePropertyName(n)] == nil {
		// Get the list of resources of the specified kind.
		list, err := q.getK8sResources(n.ResourceProperties.Kind, fieldSelector, labelSelector)
		if err != nil {
			fmt.Println("Error getting list of resources: ", err)
			return err
		}
		// merge list into resultCache
		var converted []map[string]interface{}
		for _, u := range list.Items {
			converted = append(converted, u.UnstructuredContent())
		}
		resultCache[resourcePropertyName(n)] = converted
		if err != nil {
			fmt.Println("Error marshalling results to JSON: ", err)
			return err
		}
	} else {
		fmt.Println("Resource already fetched")
	}
	resultMap[n.ResourceProperties.Name] = resultCache[resourcePropertyName(n)]
	return nil
}

func resourcePropertyName(n *NodePattern) string {
	var ns string
	if n.ResourceProperties.Properties == nil {
		return fmt.Sprintf("%s_%s", Namespace, n.ResourceProperties.Kind)
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
	return fmt.Sprintf("%s_%s_%s", ns, n.ResourceProperties.Kind, joinedPairs)
}
