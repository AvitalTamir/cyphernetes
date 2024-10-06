package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/oliveagle/jsonpath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type Node struct {
	Id        string
	Kind      string
	Name      string
	Namespace string
}

type Edge struct {
	From string
	To   string
	Type string
}

type Graph struct {
	Nodes []Node
	Edges []Edge
}

type QueryResult struct {
	Data  map[string]interface{}
	Graph Graph
}

var resultCache = make(map[string]interface{})
var resultMap = make(map[string]interface{})

func (q *QueryExecutor) Execute(ast *Expression, namespace string) (QueryResult, error) {
	if namespace != "" {
		Namespace = namespace
	}
	results := &QueryResult{
		Data: make(map[string]interface{}),
		Graph: Graph{
			Nodes: []Node{},
			Edges: []Edge{},
		},
	}
	// Iterate over the clauses in the AST.
	for _, clause := range ast.Clauses {
		switch c := clause.(type) {
		case *MatchClause:
			var filteringOccurred bool
			filteredResults := make(map[string][]map[string]interface{})

			for i := 0; i < len(c.Relationships)*2; i++ {
				filteringOccurred = false
				for _, rel := range c.Relationships {
					filtered, err := q.processRelationship(rel, c, results, filteredResults)
					if err != nil {
						return *results, err
					}
					filteringOccurred = filteringOccurred || filtered
				}
				if !filteringOccurred {
					break
				}
				// Update resultMap with filtered results for the next pass
				for k, v := range filteredResults {
					resultMap[k] = v
				}
			}

			// Process nodes
			err := q.processNodes(c, results)
			if err != nil {
				return *results, err
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
					return *results, fmt.Errorf("error marshalling patch to JSON >> %s", err)
				}

				// Apply the patch to the resources
				err = q.patchK8sResources(resultMapKey, patchJson)
				if err != nil {
					return *results, fmt.Errorf("error patching resource >> %s", err)
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
					return *results, fmt.Errorf("node identifier %s not found in result map", nodeId)
				}
				err := q.deleteK8sResources(nodeId)
				if err != nil {
					return *results, fmt.Errorf("error deleting resource >> %s", err)
				}
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
					return *results, fmt.Errorf("both nodes '%v', '%v' of relationship in create clause already exist", rel.LeftNode.ResourceProperties.Name, rel.RightNode.ResourceProperties.Name)
				}

				// TODO: create both nodes and determine the spec from the relationship instead of this:
				// If neither node exists in the match clause, error out
				if resultMap[rel.LeftNode.ResourceProperties.Name] == nil && resultMap[rel.RightNode.ResourceProperties.Name] == nil {
					return *results, fmt.Errorf("not yet supported: neither node '%s', '%s' of relationship in create clause already exist", rel.LeftNode.ResourceProperties.Name, rel.RightNode.ResourceProperties.Name)
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
					return *results, fmt.Errorf("error finding API resource >> %s", err)

				}
				foreignGVR, err := FindGVR(q.Clientset, foreignNode.ResourceProperties.Kind)
				if err != nil {
					return *results, fmt.Errorf("error finding API resource >> %s", err)
				}

				for _, resourceRelationship := range relationshipRules {
					if (strings.EqualFold(targetGVR.Resource, resourceRelationship.KindA) && strings.EqualFold(foreignGVR.Resource, resourceRelationship.KindB)) ||
						(strings.EqualFold(foreignGVR.Resource, resourceRelationship.KindA) && strings.EqualFold(targetGVR.Resource, resourceRelationship.KindB)) {
						relType = resourceRelationship.Relationship
					}
				}

				if relType == "" {
					// no relationship type found, error out
					return *results, fmt.Errorf("relationship type not found between %s and %s", targetGVR.Resource, foreignGVR.Resource)
				}

				rule, err := findRuleByRelationshipType(relType)
				if err != nil {
					return *results, fmt.Errorf("error determining relationship type >> %s", err)
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
					return *results, fmt.Errorf("relationship rule not found for %s and %s - This code path should be invalid, likely problem with rule definitions", targetGVR.Resource, foreignGVR.Resource)
				}

				var resourceTemplate map[string]interface{}
				if node.ResourceProperties.JsonData != "" {
					err = json.Unmarshal([]byte(node.ResourceProperties.JsonData), &resourceTemplate)
					if err != nil {
						fmt.Println("Error unmarshalling node JsonData: ", err)
						return *results, err
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
						return *results, fmt.Errorf("error creating resource >> %s", err)
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
						return *results, fmt.Errorf("can't create: node '%s' already exists in match clause", node.ResourceProperties.Name)
					}

					// unmarsall the node JsonData into a map
					var resourceTemplate map[string]interface{}
					err := json.Unmarshal([]byte(node.ResourceProperties.JsonData), &resourceTemplate)
					if err != nil {
						fmt.Println("Error unmarshalling node JsonData: ", err)
						return *results, err
					}

					name := getTargetK8sResourceName(resourceTemplate, node.ResourceProperties.Name, "")
					// create the resource
					err = q.createK8sResource(node, resourceTemplate, name)
					if err != nil {
						return *results, fmt.Errorf("error creating resource >> %s", err)
					}
				}
			}

		case *ReturnClause:
			nodeIds := []string{}
			for _, item := range c.Items {
				// generate a unique list of nodeIds
				nodeId := strings.Split(item.JsonPath, ".")[0]
				if !slices.Contains(nodeIds, nodeId) {
					nodeIds = append(nodeIds, nodeId)
				}
			}

			// for each nodeId, verify c.Items contains nodeId.metadata.name
			for _, nodeId := range nodeIds {
				metadataNamePath := strings.Join([]string{nodeId, "metadata.name"}, ".")
				// Check if metadataNamePath already exists in c.Items
				exists := false
				for _, item := range c.Items {
					if item.JsonPath == metadataNamePath {
						exists = true
						break
					}
				}
				// If it doesn't exist, add it
				if !exists {
					c.Items = append(c.Items, &ReturnItem{JsonPath: metadataNamePath})
				}
			}

			for _, item := range c.Items {
				nodeId := strings.Split(item.JsonPath, ".")[0]
				if resultMap[nodeId] == nil {
					return *results, fmt.Errorf("node identifier %s not found in return clause", nodeId)
				}

				pathParts := strings.Split(item.JsonPath, ".")[1:]
				pathStr := "$." + strings.Join(pathParts, ".")

				if pathStr == "$." {
					pathStr = "$"
				}

				if results.Data[nodeId] == nil {
					results.Data[nodeId] = []interface{}{}
				}
				var aggregateResult interface{}

				for idx, resource := range resultMap[nodeId].([]map[string]interface{}) {
					if len(results.Data[nodeId].([]interface{})) <= idx {
						results.Data[nodeId] = append(results.Data[nodeId].([]interface{}), make(map[string]interface{}))
					}
					currentMap := results.Data[nodeId].([]interface{})[idx].(map[string]interface{})

					result, err := jsonpath.JsonPathLookup(resource, pathStr)
					if err != nil {
						logDebug("Path not found:", item.JsonPath)
						result = nil
					}

					switch strings.ToUpper(item.Aggregate) {
					case "COUNT":
						if aggregateResult == nil {
							aggregateResult = 0
						}
						aggregateResult = aggregateResult.(int) + 1
					case "SUM":
						if result != nil {
							if aggregateResult == nil {
								aggregateResult = reflect.ValueOf(result).Interface()
							} else {
								v1 := reflect.ValueOf(aggregateResult)
								v2 := reflect.ValueOf(result)
								v1 = reflect.ValueOf(v1.Interface()).Convert(v1.Type())
								if v1.Kind() == reflect.Ptr {
									v1 = v1.Elem()
								}
								if v2.Kind() == reflect.Ptr {
									v2 = v2.Elem()
								}
								switch v1.Kind() {
								case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
									aggregateResult = v1.Int() + v2.Int()
								case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
									aggregateResult = v1.Uint() + v2.Uint()
								case reflect.Float32, reflect.Float64:
									aggregateResult = v1.Float() + v2.Float()
								default:
									// Handle unsupported types or error out
									return *results, fmt.Errorf("unsupported type for SUM: %v", v1.Kind())
								}
							}
						}
					}

					if item.Aggregate == "" {
						key := item.Alias
						if key == "" {
							if len(pathParts) > 0 {
								key = pathParts[len(pathParts)-1]
							} else {
								key = nodeId
							}
						}
						currentMap[key] = result
					}
				}
				if item.Aggregate != "" {
					if results.Data["aggregate"] == nil {
						results.Data["aggregate"] = make(map[string]interface{})
					}
					aggregateMap := results.Data["aggregate"].(map[string]interface{})

					key := item.Alias
					if key == "" {
						key = strings.ToLower(item.Aggregate) + ":" + nodeId + "." + strings.Replace(pathStr, "$.", "", 1)
					}
					aggregateMap[key] = aggregateResult
				}
			}

		default:
			return *results, fmt.Errorf("unknown clause type: %T", c)
		}
	}
	// build the graph
	q.buildGraph(results)

	// clear the result cache and result map
	resultCache = make(map[string]interface{})
	resultMap = make(map[string]interface{})
	return *results, nil
}

func (q *QueryExecutor) processRelationship(rel *Relationship, c *MatchClause, results *QueryResult, filteredResults map[string][]map[string]interface{}) (bool, error) {
	// fmt.Printf("Debug: Processing relationship: %+v\n", rel)

	// Determine relationship type and fetch related resources
	var relType RelationshipType
	if rel.LeftNode.ResourceProperties.Kind == "" || rel.RightNode.ResourceProperties.Kind == "" {
		// error out
		return false, fmt.Errorf("must specify kind for all nodes in match clause")
	}
	leftKind, err := FindGVR(q.Clientset, rel.LeftNode.ResourceProperties.Kind)
	if err != nil {
		return false, fmt.Errorf("error finding API resource >> %s", err)
	}
	rightKind, err := FindGVR(q.Clientset, rel.RightNode.ResourceProperties.Kind)
	if err != nil {
		return false, fmt.Errorf("error finding API resource >> %s", err)
	}

	if rightKind.Resource == "namespaces" || leftKind.Resource == "namespaces" {
		relType = NamespaceHasResource
	}

	if relType == "" {
		for _, resourceRelationship := range relationshipRules {
			if (strings.EqualFold(leftKind.Resource, resourceRelationship.KindA) && strings.EqualFold(rightKind.Resource, resourceRelationship.KindB)) ||
				(strings.EqualFold(rightKind.Resource, resourceRelationship.KindA) && strings.EqualFold(leftKind.Resource, resourceRelationship.KindB)) {
				relType = resourceRelationship.Relationship
			}
		}
	}

	if relType == "" {
		// no relationship type found, error out
		return false, fmt.Errorf("relationship type not found between %s and %s", leftKind, rightKind)
	}

	rule, err := findRuleByRelationshipType(relType)
	if err != nil {
		return false, fmt.Errorf("error determining relationship type >> %s", err)
	}

	// Fetch and process related resources
	for _, node := range c.Nodes {
		if node.ResourceProperties.Name == rel.LeftNode.ResourceProperties.Name || node.ResourceProperties.Name == rel.RightNode.ResourceProperties.Name {
			if results.Data[node.ResourceProperties.Name] == nil {
				err := getNodeResources(node, q, c.ExtraFilters)
				if err != nil {
					return false, err
				}
			}
		}
	}

	var resourcesA, resourcesB []map[string]interface{}
	var filteredDirection Direction

	if rule.KindA == rightKind.Resource {
		resourcesA = getResourcesFromMap(filteredResults, rel.RightNode.ResourceProperties.Name)
		resourcesB = getResourcesFromMap(filteredResults, rel.LeftNode.ResourceProperties.Name)
		filteredDirection = Left
	} else if rule.KindA == leftKind.Resource {
		resourcesA = getResourcesFromMap(filteredResults, rel.LeftNode.ResourceProperties.Name)
		resourcesB = getResourcesFromMap(filteredResults, rel.RightNode.ResourceProperties.Name)
		filteredDirection = Right
	} else {
		return false, fmt.Errorf("relationship rule not found for %s and %s - This code path should be invalid, likely problem with rule definitions", rel.LeftNode.ResourceProperties.Kind, rel.RightNode.ResourceProperties.Kind)
	}

	matchedResources := applyRelationshipRule(resourcesA, resourcesB, rule, filteredDirection)

	filteredA := len(matchedResources["right"].([]map[string]interface{})) < len(resourcesA)
	filteredB := len(matchedResources["left"].([]map[string]interface{})) < len(resourcesB)

	filteredResults[rel.RightNode.ResourceProperties.Name] = matchedResources["right"].([]map[string]interface{})
	filteredResults[rel.LeftNode.ResourceProperties.Name] = matchedResources["left"].([]map[string]interface{})

	// if resultMap[rel.RightNode.ResourceProperties.Name] already contains items, we need to check which has a smaller number of items, and use the smaller of the two lists
	// this is to ensure that we don't end up with unflitered items which should have been filtered out in the relationship rule application
	if resultMap[rel.RightNode.ResourceProperties.Name] != nil {
		if len(resultMap[rel.RightNode.ResourceProperties.Name].([]map[string]interface{})) > len(matchedResources["right"].([]map[string]interface{})) {
			resultMap[rel.RightNode.ResourceProperties.Name] = matchedResources["right"]
		}
	} else {
		resultMap[rel.RightNode.ResourceProperties.Name] = matchedResources["right"]
	}
	if resultMap[rel.LeftNode.ResourceProperties.Name] != nil {
		if len(resultMap[rel.LeftNode.ResourceProperties.Name].([]map[string]interface{})) > len(matchedResources["left"].([]map[string]interface{})) {
			resultMap[rel.LeftNode.ResourceProperties.Name] = matchedResources["left"]
		}
	} else {
		resultMap[rel.LeftNode.ResourceProperties.Name] = matchedResources["left"]
	}

	// fmt.Printf("Debug: Matched resources: %+v\n", matchedResources)

	if rightResources, ok := matchedResources["right"].([]map[string]interface{}); ok && len(rightResources) > 0 {
		for idx, rightResource := range rightResources {
			if metadata, ok := rightResource["metadata"].(map[string]interface{}); ok {
				if name, ok := metadata["name"].(string); ok {
					node := Node{
						Id:   rel.RightNode.ResourceProperties.Name,
						Kind: resultMap[rel.RightNode.ResourceProperties.Name].([]map[string]interface{})[idx]["kind"].(string),
						Name: name,
					}
					if node.Kind != "Namespace" {
						node.Namespace = getNamespaceName(metadata)
					}
					results.Graph.Nodes = append(results.Graph.Nodes, node)
				}
			}
		}
	}

	if leftResources, ok := matchedResources["left"].([]map[string]interface{}); ok && len(leftResources) > 0 {
		for idx, leftResource := range leftResources {
			if metadata, ok := leftResource["metadata"].(map[string]interface{}); ok {
				if name, ok := metadata["name"].(string); ok {
					node := Node{
						Id:   rel.LeftNode.ResourceProperties.Name,
						Kind: resultMap[rel.LeftNode.ResourceProperties.Name].([]map[string]interface{})[idx]["kind"].(string),
						Name: name,
					}
					if node.Kind != "Namespace" {
						node.Namespace = getNamespaceName(metadata)
					}
					results.Graph.Nodes = append(results.Graph.Nodes, node)
				}
			}
		}
	}

	// Only add edge if both nodes exist
	if len(matchedResources["right"].([]map[string]interface{})) > 0 && len(matchedResources["left"].([]map[string]interface{})) > 0 {
		rightNodeResources := resultMap[rel.RightNode.ResourceProperties.Name].([]map[string]interface{})
		leftNodeResources := resultMap[rel.LeftNode.ResourceProperties.Name].([]map[string]interface{})

		for _, rightNodeResource := range rightNodeResources {
			rightNodeId := fmt.Sprintf("%s/%s", rightNodeResource["kind"].(string), rightNodeResource["metadata"].(map[string]interface{})["name"].(string))
			for _, leftNodeResource := range leftNodeResources {
				leftNodeId := fmt.Sprintf("%s/%s", leftNodeResource["kind"].(string), leftNodeResource["metadata"].(map[string]interface{})["name"].(string))

				// apply the relationship rule to the two nodes
				// asign into resourceA and resourceB the right and left node resources by the rule kinds
				var resourceA []map[string]interface{}
				var resourceB []map[string]interface{}
				if rightKind.Resource == rule.KindA {
					resourceA = []map[string]interface{}{rightNodeResource}
					resourceB = []map[string]interface{}{leftNodeResource}
				} else if leftKind.Resource == rule.KindA {
					resourceA = []map[string]interface{}{leftNodeResource}
					resourceB = []map[string]interface{}{rightNodeResource}
				}
				matchedResources := applyRelationshipRule(resourceA, resourceB, rule, filteredDirection)
				if len(matchedResources["right"].([]map[string]interface{})) == 0 || len(matchedResources["left"].([]map[string]interface{})) == 0 {
					continue
				}
				results.Graph.Edges = append(results.Graph.Edges, Edge{
					From: rightNodeId,
					To:   leftNodeId,
					Type: string(relType),
				})
			}
		}
	}

	return filteredA || filteredB, nil
}

func getResourcesFromMap(filteredResults map[string][]map[string]interface{}, key string) []map[string]interface{} {
	if filtered, ok := filteredResults[key]; ok {
		return filtered
	}
	if resources, ok := resultMap[key].([]map[string]interface{}); ok {
		return resources
	}
	return nil
}

func (q *QueryExecutor) processNodes(c *MatchClause, results *QueryResult) error {
	for _, node := range c.Nodes {
		if node.ResourceProperties.Kind == "" {
			// error out
			return fmt.Errorf("must specify kind for all nodes in match clause")
		}
		debugLog("Node pattern found. Name:", node.ResourceProperties.Name, "Kind:", node.ResourceProperties.Kind)
		// check if the node has already been fetched
		if resultCache[q.resourcePropertyName(node)] == nil {
			err := getNodeResources(node, q, c.ExtraFilters)
			if err != nil {
				return fmt.Errorf("error getting node resources >> %s", err)
			}
			resources := resultMap[node.ResourceProperties.Name].([]map[string]interface{})
			for _, resource := range resources {
				metadata, ok := resource["metadata"].(map[string]interface{})
				if !ok {
					continue
				}
				node := Node{
					Id:   node.ResourceProperties.Name,
					Kind: resource["kind"].(string),
					Name: metadata["name"].(string),
				}
				if node.Kind != "Namespace" {
					node.Namespace = getNamespaceName(metadata)
				}
				results.Graph.Nodes = append(results.Graph.Nodes, node)
			}
		} else if resultMap[node.ResourceProperties.Name] == nil {
			resultMap[node.ResourceProperties.Name] = resultCache[q.resourcePropertyName(node)]
		}
	}
	return nil
}

func (q *QueryExecutor) buildGraph(result *QueryResult) {
	// fmt.Println("Debug: Building graph")
	// fmt.Printf("Debug: result.Data: %+v\n", result.Data)
	// fmt.Printf("Debug: Initial result.Graph.Edges: %+v\n", result.Graph.Edges)

	nodeMap := make(map[string]bool)
	edgeMap := make(map[string]bool)

	// Process nodes (this part remains the same)
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
			metadata, ok := resourceMap["metadata"].(map[string]interface{})
			if !ok {
				continue
			}
			name, ok := metadata["name"].(string)
			if !ok {
				continue
			}
			kind, ok := resourceMap["kind"].(string)
			if !ok {
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

			if !nodeMap[node.Id] {
				result.Graph.Nodes = append(result.Graph.Nodes, node)
				nodeMap[node.Id] = true
			}
		}
	}

	// Process edges
	newEdges := []Edge{}
	for _, edge := range result.Graph.Edges {
		edgeKey := fmt.Sprintf("%s-%s-%s", edge.From, edge.To, edge.Type)
		reverseEdgeKey := fmt.Sprintf("%s-%s-%s", edge.To, edge.From, edge.Type)

		if !edgeMap[edgeKey] && !edgeMap[reverseEdgeKey] {
			newEdges = append(newEdges, edge)
			edgeMap[edgeKey] = true
			edgeMap[reverseEdgeKey] = true
		}
	}

	result.Graph.Edges = newEdges
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
	fmt.Printf("Created %s/%s\n", gvr.Resource, name)

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
		resourceNamespace := resultMap[nodeId].([]map[string]interface{})[i]["metadata"].(map[string]interface{})["namespace"].(string)

		err = q.DynamicClient.Resource(gvr).Namespace(resourceNamespace).Delete(context.Background(), resourceName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting resource >> %v", err)
		}
		fmt.Printf("Deleted %s/%s\n", gvr.Resource, resourceName)
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
		resourceNamespace := resultMap[resultMapKey].([]map[string]interface{})[i]["metadata"].(map[string]interface{})["namespace"].(string)

		_, err = q.DynamicClient.Resource(gvr).Namespace(resourceNamespace).Patch(context.Background(), resourceName, types.JSONPatchType, []byte(patchStr), metav1.PatchOptions{})
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
			if prop.Key == "name" || prop.Key == "metadata.name" || prop.Key == `"name"` || prop.Key == `"metadata.name"` {
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
	}

	resultMap[n.ResourceProperties.Name] = resultCache[q.resourcePropertyName(n)]

	// Apply extra filters
	for _, filter := range extraFilters {
		// The first part of the key is the node name
		resultMapKey := strings.Split(filter.Key, ".")[0]
		if resultMap[resultMapKey] == nil {
			logDebug(fmt.Sprintf("node identifier %s not found in where clause", resultMapKey))
		} else if resultMapKey == n.ResourceProperties.Name {
			// The rest of the key is the JSONPath
			path := strings.Join(strings.Split(filter.Key, ".")[1:], ".")
			// Ensure the JSONPath starts with '$'
			if !strings.HasPrefix(path, "$") {
				path = "$." + path
			}

			// we'll iterate on each resource in the resultMap[node.ResourceProperties.Name] and if the resource doesn't match the filter, we'll remove it from the slice
			for j, resource := range resultMap[n.ResourceProperties.Name].([]map[string]interface{}) {
				// Drill down to create nested map structure
				result, err := jsonpath.JsonPathLookup(resource, path)
				if err != nil {
					logDebug("Path not found:", filter.Key)
					// remove the resource from the slice
					resultMap[n.ResourceProperties.Name].([]map[string]interface{})[j] = nil
					continue
				}

				// Convert result and filter.Value to comparable types
				resultValue, filterValue, err := convertToComparableTypes(result, filter.Value)
				if err != nil {
					logDebug(fmt.Sprintf("Error converting types: %v", err))
					continue
				}

				keep := false
				switch filter.Operator {
				case "EQUALS":
					keep = reflect.DeepEqual(resultValue, filterValue)
				case "NOT_EQUALS":
					keep = !reflect.DeepEqual(resultValue, filterValue)
				case "GREATER_THAN", "LESS_THAN", "GREATER_THAN_EQUALS", "LESS_THAN_EQUALS":
					if resultNum, ok := resultValue.(float64); ok {
						if filterNum, ok := filterValue.(float64); ok {
							keep = compareNumbers(resultNum, filterNum, filter.Operator)
						} else {
							logDebug(fmt.Sprintf("Invalid comparison: %v is not a number", filterValue))
						}
					} else {
						logDebug(fmt.Sprintf("Invalid comparison: %v is not a number", resultValue))
					}
				default:
					logDebug(fmt.Sprintf("Unknown operator: %s", filter.Operator))
				}

				if !keep {
					// remove the resource from the slice
					resultMap[n.ResourceProperties.Name].([]map[string]interface{})[j] = nil
				}
			}

			// remove nil values from the slice
			var filtered []map[string]interface{}
			for _, resource := range resultMap[n.ResourceProperties.Name].([]map[string]interface{}) {
				if resource != nil {
					filtered = append(filtered, resource)
				}
			}

			resultMap[n.ResourceProperties.Name] = filtered
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

func convertToComparableTypes(result, filterValue interface{}) (interface{}, interface{}, error) {
	// If both are already the same type, return them as is
	if reflect.TypeOf(result) == reflect.TypeOf(filterValue) {
		return result, filterValue, nil
	}

	// Try to convert both to float64 for numeric comparisons
	resultFloat, resultErr := toFloat64(result)
	filterFloat, filterErr := toFloat64(filterValue)

	if resultErr == nil && filterErr == nil {
		return resultFloat, filterFloat, nil
	}

	// If conversion to float64 failed, convert both to strings
	return fmt.Sprintf("%v", result), fmt.Sprintf("%v", filterValue), nil
}

func toFloat64(v interface{}) (float64, error) {
	switch v := v.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %v to float64", v)
	}
}

func compareNumbers(a, b float64, operator string) bool {
	switch operator {
	case "GREATER_THAN":
		return a > b
	case "LESS_THAN":
		return a < b
	case "GREATER_THAN_EQUALS":
		return a >= b
	case "LESS_THAN_EQUALS":
		return a <= b
	default:
		return false
	}
}
