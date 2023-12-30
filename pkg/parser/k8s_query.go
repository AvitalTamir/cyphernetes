package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/oliveagle/jsonpath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var resultCache = make(map[string]interface{})
var resultMap = make(map[string]interface{})

func (q *QueryExecutor) Execute(ast *Expression) (interface{}, error) {
	results := make(map[string]interface{})

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
				if rel.LeftNode.ResourceProperties.Kind == "" || rel.RightNode.ResourceProperties.Kind == "" {
					// error out
					return nil, fmt.Errorf("must specify kind for all nodes in match clause")
				}
				leftKind, err := FindGVR(q.Clientset, rel.LeftNode.ResourceProperties.Kind)
				if err != nil {
					return nil, fmt.Errorf("error finding API resource >> %s", err)
				}
				rightKind, err := FindGVR(q.Clientset, rel.RightNode.ResourceProperties.Kind)
				if err != nil {
					return nil, fmt.Errorf("error finding API resource >> %s", err)
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
					return nil, fmt.Errorf("error determining relationship type >> %s", err)
				}

				// Fetch and process related resources based on relationship type
				for _, node := range c.Nodes {
					if node.ResourceProperties.Name == rel.LeftNode.ResourceProperties.Name || node.ResourceProperties.Name == rel.RightNode.ResourceProperties.Name {
						if resultMap[node.ResourceProperties.Name] == nil {
							getNodeResources(node, q, c.ExtraFilters)
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
				resultMap[rel.RightNode.ResourceProperties.Name] = matchedResources["right"]
				resultMap[rel.LeftNode.ResourceProperties.Name] = matchedResources["left"]
			}

			// Iterate over the nodes in the match clause.
			for _, node := range c.Nodes {
				if node.ResourceProperties.Kind == "" {
					// error out
					return nil, fmt.Errorf("must specify kind for all nodes in match clause")
				}
				debugLog("Node pattern found. Name:", node.ResourceProperties.Name, "Kind:", node.ResourceProperties.Kind)
				// check if the node has already been fetched
				if resultCache[q.resourcePropertyName(node)] == nil {
					err := getNodeResources(node, q, c.ExtraFilters)
					if err != nil {
						return nil, fmt.Errorf("error getting node resources >> %s", err)
					}
				} else if resultMap[node.ResourceProperties.Name] == nil {
					resultMap[node.ResourceProperties.Name] = resultCache[q.resourcePropertyName(node)]
				}
			}

		case *SetClause:
			// Execute a Kubernetes update operation based on the SetClause.
			// ...
			for _, kvp := range c.KeyValuePairs {
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
					return nil, fmt.Errorf("error marshalling patch to JSON >> %s", err)
				}

				// Apply the patch to the resources
				err = q.patchK8sResources(resultMapKey, patchJson)
				if err != nil {
					return nil, fmt.Errorf("error patching resource >> %s", err)
				}

				// Retrieve the slice of maps for the resultMapKey
				if resources, ok := resultMap[resultMapKey].([]map[string]interface{}); ok {
					for idx, resource := range resources {
						// Check if the idx is within bounds
						if idx >= 0 && idx < len(resources) {
							fullPath := strings.Join(path, ".") // Construct the full path
							patchResultMap(resource, fullPath, kvp.Value)
							resources[idx] = resource // Update the specific entry in the slice
						} else {
							fmt.Printf("Index out of range for key: %s, Index: %d, Length: %d\n", resultMapKey, idx, len(resources))
							// Handle index out of range
						}
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

		case *CreateClause:
			// Same as in Match clauses, we'll first look at relationships, then nodes
			// we'll iterates over the replationships then nodes, and from each we'll extract a spec and create the resource
			// in relationships, we'll need to find the matching node in the nodes list, we'll then construct the spec from the node properties and from the relevant part of the spec that's defined in the relationship
			// in nodes, we'll just construct the spec from the node properties

			// Iterate over the relationships in the create clause.
			// Process Relationships
			for idx, rel := range c.Relationships {
				// Determine which (if any) of the nodes in the relationship have already been fetched in a match clause, and which are new creations
				var node *NodePattern
				var foreignNode *NodePattern

				// If both nodes exist in the match clause, error out
				if resultMap[rel.LeftNode.ResourceProperties.Name] != nil && resultMap[rel.RightNode.ResourceProperties.Name] != nil {
					if (node) != nil {
						return nil, fmt.Errorf("both nodes '%v', '%v' of relationship in create clause already exist", node.ResourceProperties.Name, foreignNode.ResourceProperties.Name)
					} else {
						return nil, fmt.Errorf("can't match and create node in the same expression")
					}
				}

				// TODO: create both nodes and determine the spec from the relationship instead of this:
				// If neither node exists in the match clause, error out
				if resultMap[rel.LeftNode.ResourceProperties.Name] == nil && resultMap[rel.RightNode.ResourceProperties.Name] == nil {
					return nil, fmt.Errorf("not yet supported: neither node '%s', '%s' of relationship in create clause already exist", node.ResourceProperties.Name, foreignNode.ResourceProperties.Name)
				}

				// find out whice node exists in the match clause, then use it to construct the spec according to the relationship
				if resultMap[rel.LeftNode.ResourceProperties.Name] == nil {
					node = rel.LeftNode
					foreignNode = rel.RightNode
				} else {
					node = rel.RightNode
					foreignNode = rel.LeftNode
				}

				// The foreign node is currently only a name reference, we'll need to find the matching node in the result map
				foreignNode.ResourceProperties.Kind = resultMap[foreignNode.ResourceProperties.Name].([]map[string]interface{})[0]["kind"].(string)

				var relType RelationshipType
				targetGVR, err := FindGVR(q.Clientset, node.ResourceProperties.Kind)
				if err != nil {
					return nil, fmt.Errorf("error finding API resource >> %s", err)

				}
				foreignGVR, err := FindGVR(q.Clientset, foreignNode.ResourceProperties.Kind)
				if err != nil {
					return nil, fmt.Errorf("error finding API resource >> %s", err)
				}

				for _, resourceRelationship := range relationshipRules {
					if (strings.EqualFold(targetGVR.Resource, resourceRelationship.KindA) && strings.EqualFold(foreignGVR.Resource, resourceRelationship.KindB)) ||
						(strings.EqualFold(foreignGVR.Resource, resourceRelationship.KindA) && strings.EqualFold(targetGVR.Resource, resourceRelationship.KindB)) {
						relType = resourceRelationship.Relationship
					}
				}

				if relType == "" {
					// no relationship type found, error out
					return nil, fmt.Errorf("relationship type not found between %s and %s", targetGVR.Resource, foreignGVR.Resource)
				}

				rule := findRuleByRelationshipType(relType)
				if err != nil {
					return nil, fmt.Errorf("error determining relationship type >> %s", err)
				}

				// Now according to which is the node that needs to be created, we'll construct the spec from the node properties and from the relevant part of the spec that's defined in the relationship
				// If the node to be created matches KindA in the relationship, then it's spec's nested structure described in the jsonPath in FieldA will have the value of the other node's FieldB
				// If the node to be created matches KindB in the relationship, then it's spec's nested structure described in the jsonPath in FieldB will have the value of the other node's FieldA
				// If the node to be created matches neither KindA nor KindB in the relationship, then error out
				var criteriaField string
				var foreignCriteriaField string
				var defaultPropFields []string
				var foreignDefaultPropFields []string

				if rule.KindA == targetGVR.Resource {
					criteriaField = rule.MatchCriteria[0].FieldA
					foreignCriteriaField = rule.MatchCriteria[0].FieldB

					// for each default prop, push into defaultProps and foreignDefaultProps
					for _, prop := range rule.MatchCriteria[0].DefaultProps {
						defaultPropFields = append(defaultPropFields, prop.FieldA)
						foreignDefaultPropFields = append(foreignDefaultPropFields, prop.FieldB)
					}

				} else if rule.KindA == foreignGVR.Resource {
					criteriaField = rule.MatchCriteria[0].FieldB
					foreignCriteriaField = rule.MatchCriteria[0].FieldA

					// for each default prop, push into defaultProps and foreignDefaultProps
					for _, prop := range rule.MatchCriteria[0].DefaultProps {
						defaultPropFields = append(defaultPropFields, prop.FieldB)
						foreignDefaultPropFields = append(foreignDefaultPropFields, prop.FieldA)
					}
				} else {
					// error out
					return nil, fmt.Errorf("relationship rule not found for %s and %s - This code path should be invalid, likely problem with rule definitions", targetGVR.Resource, foreignGVR.Resource)
				}

				var resourceTemplate map[string]interface{}
				if node.ResourceProperties.JsonData != "" {
					err = json.Unmarshal([]byte(node.ResourceProperties.JsonData), &resourceTemplate)
					if err != nil {
						fmt.Println("Error unmarshalling node JsonData: ", err)
						return nil, err
					}
				} else {
					resourceTemplate = make(map[string]interface{})
				}

				// loop over the resources array in the resultMap for the foreign node and create the resource
				for _, foreignResource := range resultMap[foreignNode.ResourceProperties.Name].([]map[string]interface{}) {
					var name string
					foreignSpec := resultMap[foreignNode.ResourceProperties.Name].([]map[string]interface{})[idx]

					fields := append([]string{criteriaField}, defaultPropFields...)
					foreignFields := append([]string{foreignCriteriaField}, foreignDefaultPropFields...)

					for i, jsonpath := range fields {
						var value interface{}
						if foreignFields[i] != "" {
							foreignPath := strings.Split(strings.TrimPrefix(foreignFields[i], "$."), ".")

							// Drill down to create nested map structure
							currentForeignPart := foreignSpec
							for _, part := range foreignPath {
								if currentForeignPart[part] == nil {
									// no default in foreign node, assign the relationship default if exists
									value = rule.MatchCriteria[0].DefaultProps[i-1].Default
									break
								}
								// if this is the last part, assign the value
								if part == foreignPath[len(foreignPath)-1] {
									value = currentForeignPart[part]
									break
								}
								// recurse into the path in the foreignSpec if not an array
								if _, ok := currentForeignPart[part].([]interface{}); !ok {
									currentForeignPart = currentForeignPart[part].(map[string]interface{})
								} else if strings.HasSuffix(part, "[]") {
									part = strings.TrimSuffix(part, "[]")
									// create the first element in an array
									currentForeignPart[part] = []interface{}{}
									currentForeignPart = currentForeignPart[part].([]interface{})[idx].(map[string]interface{})
								}
							}
						} else {
							// no default in foreign node, assign the relationship default if exists
							value = rule.MatchCriteria[0].DefaultProps[i-1].Default
						}
						// assign the value of the right node's FieldB to the left node's FieldA
						// iterate over fieldB after splitting it on dot (make sure to remove the '$.' if they exist in the jsonPath)
						// create the nested structure in the spec if it doesn't exist
						// assign the value to the last part of the jsonPath

						if value != nil && value != "" {
							targetField := strings.TrimPrefix(jsonpath, "$.")
							path := strings.Split(targetField, ".")
							currentPart := resourceTemplate
							for j, part := range path {
								if j == len(path)-1 {
									// Last part: assign the result
									currentPart[part] = value
								} else {
									// Intermediate parts: create nested maps
									if currentPart[part] == nil && currentPart[strings.TrimSuffix(part, "[]")] == nil {
										// if part ends with '[]', create an array and recurse into the first element
										if strings.HasSuffix(part, "[]") {
											part = strings.TrimSuffix(part, "[]")
											currentPart[part] = []interface{}{}
											currentPart[part] = append(currentPart[part].([]interface{}), make(map[string]interface{}))
											currentPart = currentPart[part].([]interface{})[idx].(map[string]interface{})
										} else {
											currentPart[part] = make(map[string]interface{})
											currentPart = currentPart[part].(map[string]interface{})
										}
									} else {
										if strings.HasSuffix(part, "[]") {
											part = strings.TrimSuffix(part, "[]")
											// if the part is an array, recurse into the first element
											currentPart = currentPart[part].([]interface{})[idx].(map[string]interface{})
										} else {
											currentPart = currentPart[part].(map[string]interface{})
										}
									}
								}
							}
						}
					}

					name = getTargetK8sResourceName(resourceTemplate, node.ResourceProperties.Name, foreignResource["metadata"].(map[string]interface{})["name"].(string))
					err = q.createK8sResource(node, resourceTemplate, name)
					if err != nil {
						return nil, fmt.Errorf("error creating resource >> %s", err)
					}
				}
			}
			// Iterate over the nodes in the create clause.
			for _, node := range c.Nodes {
				// check if node exists in a relationship in the clause, if so, ignore it
				ignoreNode := false
				for _, rel := range c.Relationships {
					if node.ResourceProperties.Name == rel.LeftNode.ResourceProperties.Name || node.ResourceProperties.Name == rel.RightNode.ResourceProperties.Name {
						ignoreNode = true
						break
					}
				}

				if !ignoreNode {
					// check if the node has already been fetched, if so, error out
					if resultMap[node.ResourceProperties.Name] != nil {
						return nil, fmt.Errorf("can't create: node '%s' already exists in match clause", node.ResourceProperties.Name)
					}

					// unmarsall the node JsonData into a map
					var resourceTemplate map[string]interface{}
					err := json.Unmarshal([]byte(node.ResourceProperties.JsonData), &resourceTemplate)
					if err != nil {
						fmt.Println("Error unmarshalling node JsonData: ", err)
						return nil, err
					}

					name := getTargetK8sResourceName(resourceTemplate, node.ResourceProperties.Name, "")
					// create the resource
					err = q.createK8sResource(node, resourceTemplate, name)
					if err != nil {
						return nil, fmt.Errorf("error creating resource >> %s", err)
					}
				}
			}

		case *ReturnClause:
			for _, jsonPath := range c.JsonPaths {
				// The first part of the key is the node name
				nodeId := strings.Split(jsonPath, ".")[0]
				if resultMap[nodeId] == nil {
					return nil, fmt.Errorf("node identifier %s not found in return clause", nodeId)
				}

				// The rest of the key is the JSONPath
				pathParts := strings.Split(jsonPath, ".")[1:]

				var pathStr string
				if len(pathParts) == 0 {
					pathParts = append(pathParts, "$")
					pathStr = "$"
				} else {
					pathStr = strings.Join(pathParts, ".")
				}

				// Ensure the JSONPath starts with '$'
				if !strings.HasPrefix(pathStr, "$") {
					pathStr = "$." + pathStr
				}

				// Create a map for the node's resources if it's empty
				if results[nodeId] == nil {
					results[nodeId] = []interface{}{}
				}

				// Iterate over the resources in the result map
				for idx, resource := range resultMap[nodeId].([]map[string]interface{}) {
					// if it doesn't exist, create a new empty slice in results[nodeId][idx]

					if len(results[nodeId].([]interface{})) <= idx {
						results[nodeId] = append(results[nodeId].([]interface{}), make(map[string]interface{}))
					}
					currentMap := results[nodeId].([]interface{})[idx].(map[string]interface{})

					// assign
					// Drill down to create nested map structure
					for i, part := range pathParts {
						if i == len(pathParts)-1 {
							// Last part: assign the result
							result, err := jsonpath.JsonPathLookup(resource, pathStr)
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
			}

		default:
			return nil, fmt.Errorf("unknown clause type: %T", c)
		}
	}
	// clear the result cache and result map
	resultCache = make(map[string]interface{})
	resultMap = make(map[string]interface{})
	return results, nil
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

func (q *QueryExecutor) createK8sResource(node *NodePattern, template map[string]interface{}, name string) error {
	// Look up the resource kind and name in the cache
	gvr, err := FindGVR(q.Clientset, node.ResourceProperties.Kind)
	if err != nil {
		return fmt.Errorf("error finding API resource >> %v", err)
	}
	kind := q.getSingularNameForGVR(gvr)
	if kind == "" {
		return fmt.Errorf("error finding singular name for resource >> %v", err)
	}

	// Construct the resource from the spec
	//resource := make(map[string]interface{})
	resource := template
	resource["apiVersion"] = gvr.GroupVersion().String()
	resource["kind"] = kind
	resource["metadata"] = make(map[string]interface{})
	resource["metadata"].(map[string]interface{})["name"] = name
	resource["metadata"].(map[string]interface{})["namespace"] = Namespace

	// Create the resource
	_, err = q.DynamicClient.Resource(gvr).Namespace(Namespace).Create(context.Background(), &unstructured.Unstructured{Object: resource}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	fmt.Printf("Created '%s/%s'\n", gvr.Resource, name)

	return nil
}

func (q *QueryExecutor) getSingularNameForGVR(gvr schema.GroupVersionResource) string {
	// Get the singular name for the resource
	// This is a workaround for the fact that the k8s API doesn't provide a way to get the singular name
	// See
	apiResourceList, err := q.Clientset.DiscoveryClient.ServerPreferredResources()
	if err != nil {
		panic(err.Error())
	}
	for _, resourceGroup := range apiResourceList {
		for _, resource := range resourceGroup.APIResources {
			if resource.Name == gvr.Resource {
				return resource.Kind
			}
		}
	}

	return ""
}

func (q *QueryExecutor) deleteK8sResources(nodeId string) error {
	resources := resultMap[nodeId].([]map[string]interface{})

	for i := range resources {
		// Look up the resource kind and name in the cache
		gvr, err := FindGVR(q.Clientset, resources[i]["kind"].(string))
		if err != nil {
			return fmt.Errorf("error finding API resource >> %v", err)
		}
		resourceName := resultMap[nodeId].([]map[string]interface{})[i]["metadata"].(map[string]interface{})["name"].(string)

		err = q.DynamicClient.Resource(gvr).Namespace(Namespace).Delete(context.Background(), resourceName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting resource >> %v", err)
		}
		fmt.Printf("Deleted '%s/%s'\n", gvr.Resource, resourceName)
	}

	// remove the resource from the result map
	delete(resultMap, nodeId)
	return nil
}

func (q *QueryExecutor) patchK8sResources(resultMapKey string, patch []byte) error {
	resources := resultMap[resultMapKey].([]map[string]interface{})

	// in patch, replace regex \[\d+\] with \/$1\/
	// this is to support patching arrays
	patchStr := string(patch)
	// now the regex replace
	re := regexp.MustCompile(`\[(\d+)\]`)
	patchStr = re.ReplaceAllString(patchStr, "/$1")

	for i := range resources {
		// Look up the resource kind and name in the cache
		gvr, err := FindGVR(q.Clientset, resources[i]["kind"].(string))
		if err != nil {
			return fmt.Errorf("error finding API resource >> %v", err)
		}
		resourceName := resultMap[resultMapKey].([]map[string]interface{})[i]["metadata"].(map[string]interface{})["name"].(string)

		_, err = q.DynamicClient.Resource(gvr).Namespace(Namespace).Patch(context.Background(), resourceName, types.JSONPatchType, []byte(patchStr), metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("error patching resource >> %v", err)
		}
		fmt.Printf("Patched %s/%s\n", gvr.Resource, resourceName)
	}
	return nil
}

func getNodeResources(n *NodePattern, q *QueryExecutor, extraFilters []*KeyValuePair) (err error) {
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
	var hasLabelSelector bool

	if n.ResourceProperties.Properties != nil {
		for _, prop := range n.ResourceProperties.Properties.PropertyList {
			if prop.Key == "name" || prop.Key == "metadata.name" {
				fieldSelector += fmt.Sprintf("metadata.name=%s,", prop.Value)
				hasNameSelector = true
			} else {
				hasLabelSelector = true
				labelSelector += fmt.Sprintf("%s=%s,", prop.Key, prop.Value)
			}
		}
		fieldSelector = strings.TrimSuffix(fieldSelector, ",")
		labelSelector = strings.TrimSuffix(labelSelector, ",")
	}
	if hasNameSelector && hasLabelSelector {
		// both name and label selectors are specified, error out
		return fmt.Errorf("the 'name' selector can be used by itself or combined with 'namespace', but not with other label selectors")
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

	// Apply extra filters
	for _, filter := range extraFilters {
		// The first part of the key is the node name
		resultMapKey := strings.Split(filter.Key, ".")[0]
		if resultMap[resultMapKey] == nil {
			return fmt.Errorf("node identifier %s not found in where clause", resultMapKey)
		} else if resultMapKey == n.ResourceProperties.Name {
			// The rest of the key is the JSONPath
			path := strings.Join(strings.Split(filter.Key, ".")[1:], ".")
			// Ensure the JSONPath starts with '$'
			if !strings.HasPrefix(path, "$") {
				path = "$." + path
			}

			// we'll iterate on each resource in the resultMap[node.ResourceProperties.Name] and if the resource doesn't match the filter, we'll remove it from the slice
			removedCount := 0
			for j, resource := range resultMap[n.ResourceProperties.Name].([]map[string]interface{}) {
				// Drill down to create nested map structure
				result, err := jsonpath.JsonPathLookup(resource, path)
				if err != nil {
					logDebug("Path not found:", filter.Key)
					// Remove the resource at index j from the slice
					resultMap[n.ResourceProperties.Name] = append(resultMap[n.ResourceProperties.Name].([]map[string]interface{})[:j-removedCount], resultMap[n.ResourceProperties.Name].([]map[string]interface{})[j-removedCount+1:]...)
					removedCount++
				}
				// convert the result and the value to strings and compare them
				resultStr := fmt.Sprintf("%v", result)
				valueStr := fmt.Sprintf("%v", filter.Value)

				if resultStr != valueStr {
					// remove the resource from the slice
					resultMap[n.ResourceProperties.Name] = append(resultMap[n.ResourceProperties.Name].([]map[string]interface{})[:j-removedCount], resultMap[n.ResourceProperties.Name].([]map[string]interface{})[j-removedCount+1:]...)
					removedCount++
				}
			}
		}
	}

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

	// Check if the nextLevel contains the regex for an array index
	re := regexp.MustCompile(`\[(\d+)\]`)
	if re.MatchString(nextLevel) {
		// If the next level is an array index, we want to recurse into the array
		index := re.FindStringSubmatch(nextLevel)[1]
		idx, err := strconv.Atoi(index)
		if err != nil {
			fmt.Println("Error converting index to int: ", err)
			return
		}
		nextLevel = strings.TrimSuffix(nextLevel, "["+index+"]")
		// If the next level is an array, continue patching
		if nextArray, ok := result[nextLevel].([]interface{}); ok {
			patchResultMap(nextArray[idx].(map[string]interface{}), remainingPath, newValue)
		} else {
			// If the next level is not an array, it needs to be created
			newArray := make([]interface{}, 0)
			result[nextLevel] = newArray
			patchResultMap(newArray[idx].(map[string]interface{}), remainingPath, newValue)
		}
	} else if nextMap, ok := result[nextLevel].(map[string]interface{}); ok {
		// If the next level is a map, continue patching
		patchResultMap(nextMap, remainingPath, newValue)
	} else {
		// If the next level is not a map, it needs to be created
		newMap := make(map[string]interface{})
		result[nextLevel] = newMap
		patchResultMap(newMap, remainingPath, newValue)
	}
}
