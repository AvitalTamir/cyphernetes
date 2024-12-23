package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"github.com/avitaltamir/jsonpath"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
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

type QueryExecutor struct {
	provider       provider.Provider
	requestChannel chan *apiRequest
	semaphore      chan struct{}
}

// Add these at the top of the file, after the imports
var (
	// Global configuration variables
	Namespace     string
	LogLevel      string
	AllNamespaces bool
	CleanOutput   bool
	NoColor       bool
)

// Add the apiRequest type definition
type apiRequest struct{}

func NewQueryExecutor(p provider.Provider) (*QueryExecutor, error) {
	if p == nil {
		return nil, fmt.Errorf("provider cannot be nil")
	}

	return &QueryExecutor{
		provider:       p,
		requestChannel: make(chan *apiRequest),
		semaphore:      make(chan struct{}, 1),
	}, nil
}

func (q *QueryExecutor) Execute(ast *Expression, namespace string) (QueryResult, error) {
	if len(ast.Contexts) > 0 {
		return ExecuteMultiContextQuery(ast, namespace)
	}
	return q.ExecuteSingleQuery(ast, namespace)
}

func (q *QueryExecutor) ExecuteSingleQuery(ast *Expression, namespace string) (QueryResult, error) {
	if AllNamespaces {
		Namespace = ""
		AllNamespaces = false // to reset value
	} else if namespace != "" {
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
			for _, kvp := range c.KeyValuePairs {
				resultMapKey := strings.Split(kvp.Key, ".")[0]
				path := []string{}
				parts := strings.Split(kvp.Key, ".")
				for i := 1; i < len(parts); i++ {
					if i > 1 && strings.HasSuffix(parts[i-1], "\\") {
						// Combine this part with the previous one, removing the backslash
						path[len(path)-1] = path[len(path)-1][:len(path[len(path)-1])-1] + "." + strings.ReplaceAll(parts[i], "/", "~1")
					} else {
						path = append(path, strings.ReplaceAll(parts[i], "/", "~1"))
					}
				}

				resources := resultMap[resultMapKey].([]map[string]interface{})
				for _, resource := range resources {
					// Create a single patch that works with the existing structure
					patches := createCompatiblePatch(path, kvp.Value)

					// Marshal the patches to JSON
					patchJSON, err := json.Marshal(patches)
					if err != nil {
						return *results, fmt.Errorf("error marshalling patches: %s", err)
					}

					// Apply the patches to the resource
					err = q.PatchK8sResource(resource, patchJSON)
					if err != nil {
						return *results, fmt.Errorf("error patching resource: %s", err)
					}

					// Update the resultMap
					updateResultMap(resource, path, kvp.Value)
				}
			}

		case *DeleteClause:
			// Execute a Kubernetes delete operation based on the DeleteClause.
			for _, nodeId := range c.NodeIds {
				// make sure the identifier is a key in the result map
				if resultMap[nodeId] == nil {
					return *results, fmt.Errorf("node identifier %s not found in result map", nodeId)
				}

				// Get the resources to delete
				resources := resultMap[nodeId].([]map[string]interface{})
				for _, resource := range resources {
					kind := resource["kind"].(string)
					metadata := resource["metadata"].(map[string]interface{})
					name := metadata["name"].(string)
					namespace := getNamespaceName(metadata)

					err := q.provider.DeleteK8sResources(kind, name, namespace)
					if err != nil {
						return *results, fmt.Errorf("error deleting resource %s/%s: %v", kind, name, err)
					}
				}

				// Remove from result map after successful deletion
				delete(resultMap, nodeId)
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
				targetGVR, err := q.findGVR(node.ResourceProperties.Kind)
				if err != nil {
					return *results, fmt.Errorf("error finding API resource >> %s", err)

				}
				foreignGVR, err := q.findGVR(foreignNode.ResourceProperties.Kind)
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
					err = q.provider.CreateK8sResource(
						node.ResourceProperties.Kind,
						name,
						Namespace,
						resourceTemplate,
					)
					if err != nil {
						return *results, fmt.Errorf("error creating resource >> %v", err)
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
					err = q.provider.CreateK8sResource(
						node.ResourceProperties.Kind,
						name,
						Namespace,
						resourceTemplate,
					)
					if err != nil {
						return *results, fmt.Errorf("error creating resource >> %v", err)
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

			// Add a "name" property to each node
			for _, nodeId := range nodeIds {
				metadataNamePath := strings.Join([]string{nodeId, "metadata.name"}, ".")
				c.Items = append(c.Items, &ReturnItem{JsonPath: metadataNamePath, Alias: "name"})
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
					// Ensure that the results.Data[nodeId] slice has enough elements to store the current resource.
					// If the current index (idx) is beyond the current length of the slice,
					// append a new empty map to the slice to accommodate the new data.
					if len(results.Data[nodeId].([]interface{})) <= idx {
						results.Data[nodeId] = append(results.Data[nodeId].([]interface{}), make(map[string]interface{}))
					}
					currentMap := results.Data[nodeId].([]interface{})[idx].(map[string]interface{})

					result, err := jsonpath.JsonPathLookup(resource, pathStr)
					if err != nil {
						// logDebug("Path not found:", item.JsonPath)
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

								isCPUResource := strings.Contains(pathStr, "resources.limits.cpu") || strings.Contains(pathStr, "resources.requests.cpu")
								isMemoryResource := strings.Contains(pathStr, "resources.limits.memory") || strings.Contains(pathStr, "resources.requests.memory")

								switch v1.Kind() {
								case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
									aggregateResult = v1.Int() + v2.Int()
								case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
									aggregateResult = v1.Uint() + v2.Uint()
								case reflect.Float32, reflect.Float64:
									aggregateResult = v1.Float() + v2.Float()
								case reflect.String:
									if isCPUResource {
										v1Cpu, err := convertToMilliCPU(v1.String())
										if err != nil {
											return *results, fmt.Errorf("error processing cpu resources value: %v", err)
										}
										v2Cpu, err := convertToMilliCPU(v2.String())
										if err != nil {
											return *results, fmt.Errorf("error processing cpu resources value: %v", err)
										}

										aggregateResult = convertMilliCPUToStandard(v1Cpu + v2Cpu)
									} else if isMemoryResource {
										v1Mem, err := convertMemoryToBytes(v1.String())
										if err != nil {
											return *results, fmt.Errorf("error processing memory resources value: %v", err)
										}
										v2Mem, err := convertMemoryToBytes(v2.String())
										if err != nil {
											return *results, fmt.Errorf("error processing memory resources value: %v", err)
										}

										aggregateResult = convertBytesToMemory(v1Mem + v2Mem)
									}
								case reflect.Slice:
									v1Strs, err := convertToStringSlice(v1)
									if err != nil {
										return *results, fmt.Errorf("error converting v1 to string slice: %v", err)
									}

									v2Strs, err := convertToStringSlice(v2)
									if err != nil {
										return *results, fmt.Errorf("error converting v2 to string slice: %v", err)
									}

									if isCPUResource {
										v1CpuSum, err := sumMilliCPU(v1Strs)
										if err != nil {
											return *results, fmt.Errorf("error processing v1 cpu value: %v", err)
										}

										v2CpuSum, err := sumMilliCPU(v2Strs)
										if err != nil {
											return *results, fmt.Errorf("error processing v2 cpu value: %v", err)
										}

										aggregateResult = []string{convertMilliCPUToStandard(v1CpuSum + v2CpuSum)}
									} else if isMemoryResource {
										v1MemSum, err := sumMemoryBytes(v1Strs)
										if err != nil {
											return *results, fmt.Errorf("error processing v1 memory value: %v", err)
										}

										v2MemSum, err := sumMemoryBytes(v2Strs)
										if err != nil {
											return *results, fmt.Errorf("error processing v2 memory value: %v", err)
										}

										aggregateResult = []string{convertBytesToMemory(v1MemSum + v2MemSum)}
									}
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
							if len(pathParts) == 1 {
								key = pathParts[0]
							} else if len(pathParts) > 1 {
								nestedMap := currentMap
								for i := 0; i < len(pathParts)-1; i++ {
									if _, exists := nestedMap[pathParts[i]]; !exists {
										nestedMap[pathParts[i]] = make(map[string]interface{})
									}
									nestedMap = nestedMap[pathParts[i]].(map[string]interface{})
								}
								nestedMap[pathParts[len(pathParts)-1]] = result
								continue
							} else {
								key = "$"
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

					if slice, ok := aggregateResult.([]interface{}); ok && len(slice) == 0 {
						aggregateResult = nil
					} else if strSlice, ok := aggregateResult.([]string); ok && len(strSlice) == 1 {
						aggregateResult = strSlice[0]
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
	// logDebug(fmt.Sprintf("Processing relationship: %+v\n", rel))

	// Determine relationship type and fetch related resources
	var relType RelationshipType
	if rel.LeftNode.ResourceProperties.Kind == "" || rel.RightNode.ResourceProperties.Kind == "" {
		// error out
		return false, fmt.Errorf("must specify kind for all nodes in match clause")
	}
	leftKind, err := q.findGVR(rel.LeftNode.ResourceProperties.Kind)
	if err != nil {
		return false, fmt.Errorf("error finding API resource >> %s", err)
	}
	rightKind, err := q.findGVR(rel.RightNode.ResourceProperties.Kind)
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

	// Update resultMap with filtered results
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

	// Add nodes and edges based on the matched resources
	rightResources := matchedResources["right"].([]map[string]interface{})
	leftResources := matchedResources["left"].([]map[string]interface{})

	// Add nodes
	for _, rightResource := range rightResources {
		if metadata, ok := rightResource["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				node := Node{
					Id:   rel.RightNode.ResourceProperties.Name,
					Kind: rightResource["kind"].(string),
					Name: name,
				}
				if node.Kind != "Namespace" {
					node.Namespace = getNamespaceName(metadata)
				}
				results.Graph.Nodes = append(results.Graph.Nodes, node)
			}
		}
	}

	for _, leftResource := range leftResources {
		if metadata, ok := leftResource["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				node := Node{
					Id:   rel.LeftNode.ResourceProperties.Name,
					Kind: leftResource["kind"].(string),
					Name: name,
				}
				if node.Kind != "Namespace" {
					node.Namespace = getNamespaceName(metadata)
				}
				results.Graph.Nodes = append(results.Graph.Nodes, node)
			}
		}
	}

	// Add edges between matched resources
	for _, rightResource := range rightResources {
		rightNodeId := fmt.Sprintf("%s/%s", rightResource["kind"].(string), rightResource["metadata"].(map[string]interface{})["name"].(string))
		for _, leftResource := range leftResources {
			leftNodeId := fmt.Sprintf("%s/%s", leftResource["kind"].(string), leftResource["metadata"].(map[string]interface{})["name"].(string))
			results.Graph.Edges = append(results.Graph.Edges, Edge{
				From: rightNodeId,
				To:   leftNodeId,
				Type: string(relType),
			})
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
	// Process each node in the match clause
	for _, node := range c.Nodes {
		// Skip if the node has already been processed
		if resultMap[node.ResourceProperties.Name] != nil {
			continue
		}

		// Get the extra filters from the match clause
		var extraFilters []*KeyValuePair
		for _, filter := range c.ExtraFilters {
			// Only include filters that apply to this node
			if strings.HasPrefix(filter.Key, node.ResourceProperties.Name+".") {
				// Remove the node name prefix from the filter key
				filterCopy := *filter
				// Remove the node identifier and keep only the path
				filterCopy.Key = strings.TrimPrefix(filter.Key, node.ResourceProperties.Name+".")
				extraFilters = append(extraFilters, &filterCopy)
			}
		}

		// Get resources for this node with the extra filters
		err := getNodeResources(node, q, extraFilters)
		if err != nil {
			return fmt.Errorf("error getting resources for node %s: %v", node.ResourceProperties.Name, err)
		}

		// Add to graph
		if resources, ok := resultMap[node.ResourceProperties.Name].([]map[string]interface{}); ok {
			for _, resource := range resources {
				metadata := resource["metadata"].(map[string]interface{})
				results.Graph.Nodes = append(results.Graph.Nodes, Node{
					Id:        node.ResourceProperties.Name,
					Kind:      node.ResourceProperties.Kind,
					Name:      metadata["name"].(string),
					Namespace: getNamespaceName(metadata),
				})
			}
		}
	}
	return nil
}

func (q *QueryExecutor) buildGraph(result *QueryResult) {
	// logDebug(fmt.Sprintln("Building graph"))
	// logDebug(fmt.Sprintf("result.Data: %+v\n", result.Data))
	// logDebug(fmt.Sprintf("Initial result.Graph.Edges: %+v\n", result.Graph.Edges))

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

func (q *QueryExecutor) resourcePropertyName(n *NodePattern) string {
	var ns string

	gvr, err := q.provider.FindGVR(n.ResourceProperties.Kind)
	if err != nil {
		fmt.Println("Error finding API resource: ", err)
		return ""
	}

	if n.ResourceProperties.Properties == nil {
		return fmt.Sprintf("%s_%s", Namespace, gvr.Resource)
	}

	for _, prop := range n.ResourceProperties.Properties.PropertyList {
		if prop.Key == "namespace" || prop.Key == "metadata.namespace" {
			ns = prop.Value.(string)
			break
		}
	}

	if ns == "" {
		ns = Namespace
	}

	return fmt.Sprintf("%s_%s", ns, gvr.Resource)
}

func convertToComparableTypes(result, filterValue interface{}) (interface{}, interface{}, error) {
	// Handle null value comparisons
	if filterValue == nil {
		return result, nil, nil
	}

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

func createCompatiblePatch(path []string, value interface{}) []interface{} {
	// Create a JSON Patch operation
	patch := map[string]interface{}{
		"op":    "replace",
		"path":  "/" + strings.Join(path, "/"),
		"value": value,
	}

	return []interface{}{patch}
}

func updateResultMap(resource map[string]interface{}, path []string, value interface{}) {
	current := resource
	for i, key := range path {
		if i == len(path)-1 {
			current[key] = value
			return
		}
		if _, ok := current[key]; !ok {
			current[key] = make(map[string]interface{})
		}
		current = current[key].(map[string]interface{})
	}
}

func (q *QueryExecutor) PatchK8sResource(resource map[string]interface{}, patchJSON []byte) error {
	// Get the resource details
	name := resource["metadata"].(map[string]interface{})["name"].(string)
	namespace := ""
	if ns, ok := resource["metadata"].(map[string]interface{})["namespace"]; ok {
		namespace = ns.(string)
	}
	kind := resource["kind"].(string)

	return q.provider.PatchK8sResource(kind, name, namespace, patchJSON)
}

// convertToMilliCPU converts a CPU value string to milliCPU (integer format).
// It handles both standard CPU values (e.g., "1", "0.5") and milliCPU values (e.g., "500m").
func convertToMilliCPU(cpu string) (int, error) {
	// Check if the value is in milliCPU format
	if strings.HasSuffix(cpu, "m") {
		// Trim the "m" suffix and convert the remaining string to an integer
		milliCPU, err := strconv.Atoi(strings.TrimSuffix(cpu, "m"))
		if err != nil {
			return 0, fmt.Errorf("invalid milliCPU value: %s", cpu)
		}
		return milliCPU, nil
	}

	// Convert to base unit (milliCPU) if no "m" suffix
	standardCPU, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid standard CPU value: %s", cpu)
	}

	return int(standardCPU * 1000), nil
}

// convertMilliCPUToStandard converts a CPU value in milliCPU to the standard notation (integer or float)
// if it's more readable. If it's less than 1000m, it returns the value in milliCPU format.
func convertMilliCPUToStandard(milliCPU int) string {
	// If the value is 1000m or greater, convert to standard CPU notation
	if milliCPU >= 1000 {
		// Convert to standard CPU by dividing by 1000 and check for decimal points
		standardCPU := float64(milliCPU) / 1000.0

		// If the value is a whole number (e.g., 2000m becomes 2), format as an integer
		if standardCPU == float64(int(standardCPU)) {
			return strconv.Itoa(int(standardCPU))
		}

		// Otherwise, format as a float, then drop the unnecessary trailing 0's
		standardCPU_str := strings.TrimRight(fmt.Sprintf("%.3f", standardCPU), "0")

		if strings.HasSuffix(standardCPU_str, ".") {
			standardCPU_str = strings.TrimRight(standardCPU_str, ".")
		}

		return standardCPU_str
	}

	// If less than 1000m, return the value in milliCPU format with the "m" suffix
	return strconv.Itoa(milliCPU) + "m"
}

// convertMemoryToBytes takes a memory string like "500M" or "2Gi"
// and returns the corresponding value in bytes.
func convertMemoryToBytes(mem string) (int64, error) {
	// Suffixes for power-of-10 (decimal) memory units in Kubernetes
	suffixesDecimal := map[string]int64{
		"E": 1e18, // Exabyte
		"P": 1e15, // Petabyte
		"T": 1e12, // Terabyte
		"G": 1e9,  // Gigabyte
		"M": 1e6,  // Megabyte
		"k": 1e3,  // Kilobyte (lowercase for kilobytes in decimal)
	}

	// Suffixes for power-of-2 (binary) memory units
	suffixesBinary := map[string]int64{
		"Ei": 1 << 60, // Exbibyte (2^60)
		"Pi": 1 << 50, // Pebibyte (2^50)
		"Ti": 1 << 40, // Tebibyte (2^40)
		"Gi": 1 << 30, // Gibibyte (2^30)
		"Mi": 1 << 20, // Mebibyte (2^20)
		"Ki": 1 << 10, // Kibibyte (2^10)
	}

	// Check for power-of-2 suffixes first (Ei, Pi, Ti, etc.)
	for suffix, multiplier := range suffixesBinary {
		if strings.HasSuffix(mem, suffix) {
			numberStr := strings.TrimSuffix(mem, suffix)
			number, err := strconv.ParseFloat(numberStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number format: %v", err)
			}
			return int64(number * float64(multiplier)), nil
		}
	}

	// Check for power-of-10 suffixes next (E, P, T, etc.)
	for suffix, multiplier := range suffixesDecimal {
		if strings.HasSuffix(mem, suffix) {
			numberStr := strings.TrimSuffix(mem, suffix)
			number, err := strconv.ParseFloat(numberStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number format: %v", err)
			}
			return int64(number * float64(multiplier)), nil
		}
	}

	// If no suffix is found, assume it's in bytes
	number, err := strconv.ParseFloat(mem, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory format: %v", err)
	}
	return int64(number), nil
}

// convertBytesToMemory converts a value in bytes to the closest readable unit,
// supporting both decimal (e.g., kB, MB) and binary (e.g., KiB, MiB) units.
func convertBytesToMemory(bytes int64) string {
	// Binary units (power-of-2)
	binaryUnits := []struct {
		suffix     string
		multiplier int64
	}{
		{"Ei", 1 << 60}, // Exbibyte
		{"Pi", 1 << 50}, // Pebibyte
		{"Ti", 1 << 40}, // Tebibyte
		{"Gi", 1 << 30}, // Gibibyte
		{"Mi", 1 << 20}, // Mebibyte
		{"Ki", 1 << 10}, // Kibibyte
	}

	// Decimal units (power-of-10)
	decimalUnits := []struct {
		suffix     string
		multiplier int64
	}{
		{"E", 1e18}, // Exabyte
		{"P", 1e15}, // Petabyte
		{"T", 1e12}, // Terabyte
		{"G", 1e9},  // Gigabyte
		{"M", 1e6},  // Megabyte
		{"k", 1e3},  // Kilobyte
	}

	// First check for decimal units (power-of-10) exactly
	for _, unit := range decimalUnits {
		if bytes == unit.multiplier {
			return fmt.Sprintf("1.0%s", unit.suffix)
		}
	}

	// Then check for binary units (power-of-two)
	for _, unit := range binaryUnits {
		if bytes >= unit.multiplier {
			value := float64(bytes) / float64(unit.multiplier)
			return fmt.Sprintf("%.1f%s", value, unit.suffix)
		}
	}

	// If no binary unit applies, check for decimal units (power-of-ten)
	for _, unit := range decimalUnits {
		if bytes >= unit.multiplier {
			value := float64(bytes) / float64(unit.multiplier)
			return fmt.Sprintf("%.1f%s", value, unit.suffix)
		}
	}

	// If no unit applies, return the value in bytes
	return fmt.Sprintf("%d", bytes)
}

func convertToStringSlice(v1 reflect.Value) ([]string, error) {
	if v1.Kind() != reflect.Slice {
		return nil, fmt.Errorf("input is not a slice")
	}

	length := v1.Len()
	result := make([]string, length)

	for i := 0; i < length; i++ {
		elem := v1.Index(i)

		// If the element is directly a string
		if elem.Kind() == reflect.String {
			result[i] = elem.String()
		} else {
			// Try to convert the element to a string
			if elem.CanInterface() {
				switch v := elem.Interface().(type) {
				case string:
					result[i] = v
				case fmt.Stringer:
					result[i] = v.String()
				default:
					// As a last resort, use fmt.Sprint
					result[i] = fmt.Sprint(v)
				}
			} else {
				return nil, fmt.Errorf("cannot convert element at index %d to string", i)
			}
		}
	}

	return result, nil
}

func sumMilliCPU(cpuStrs []string) (int, error) {
	cpuSum := 0
	for _, cpuStr := range cpuStrs {
		cpuVal, err := convertToMilliCPU(cpuStr)
		if err != nil {
			return 0, fmt.Errorf("error processing CPU value: %v", err)
		}

		cpuSum += cpuVal
	}

	return cpuSum, nil
}

func sumMemoryBytes(memStrs []string) (int64, error) {
	memSum := int64(0)
	for _, memStr := range memStrs {
		memVal, err := convertMemoryToBytes(memStr)
		if err != nil {
			return 0, fmt.Errorf("error processing Memory value: %v", err)
		}

		memSum += memVal
	}

	return memSum, nil
}

// Add new function to handle multi-context execution
func ExecuteMultiContextQuery(ast *Expression, namespace string) (QueryResult, error) {
	results := &QueryResult{
		Data: make(map[string]interface{}),
		Graph: Graph{
			Nodes: []Node{},
			Edges: []Edge{},
		},
	}

	// Process each context
	for _, context := range ast.Contexts {
		// Create a new provider for this context
		contextExecutor, err := GetContextQueryExecutor(context)
		if err != nil {
			return QueryResult{}, fmt.Errorf("error getting context executor: %v", err)
		}

		// Execute the query in this context
		contextResult, err := contextExecutor.ExecuteSingleQuery(ast, namespace)
		if err != nil {
			return QueryResult{}, fmt.Errorf("error executing query in context %s: %v", context, err)
		}

		// Prefix the results with the context name
		for key, value := range contextResult.Data {
			results.Data[context+"_"+key] = value
		}

		// Add nodes and edges to the graph
		results.Graph.Nodes = append(results.Graph.Nodes, contextResult.Graph.Nodes...)
		results.Graph.Edges = append(results.Graph.Edges, contextResult.Graph.Edges...)
	}

	return *results, nil
}

func (q *QueryExecutor) findGVR(kind string) (schema.GroupVersionResource, error) {
	return q.provider.FindGVR(kind)
}

func (q *QueryExecutor) GetOpenAPIResourceSpecs() (map[string][]string, error) {
	specs, err := q.provider.GetOpenAPIResourceSpecs()
	if err != nil {
		return nil, fmt.Errorf("error getting OpenAPI resource specs: %w", err)
	}
	return specs, nil
}

// Add these variables at the top with the other vars
var (
	executorInstance *QueryExecutor
	contextExecutors map[string]*QueryExecutor
	once             sync.Once
	GvrCache         map[string]schema.GroupVersionResource
	ResourceSpecs    map[string][]string
	executorsLock    sync.RWMutex
)

func GetQueryExecutorInstance(p provider.Provider) *QueryExecutor {
	once.Do(func() {
		if p == nil {
			fmt.Println("Error creating query executor: executor error")
			return
		}

		executor, err := NewQueryExecutor(p)
		if err != nil {
			fmt.Printf("Error creating QueryExecutor instance: %v\n", err)
			return
		}

		executorInstance = executor
		contextExecutors = make(map[string]*QueryExecutor)

		// Initialize GVR cache
		if err := InitGVRCache(p); err != nil {
			fmt.Printf("Error initializing GVR cache: %v\n", err)
			return
		}

		// Initialize resource specs
		if err := InitResourceSpecs(p); err != nil {
			fmt.Printf("Error initializing resource specs: %v\n", err)
			return
		}

		// Initialize relationships
		InitializeRelationships(ResourceSpecs)
	})
	return executorInstance
}

// Add this getter method for the provider
func (q *QueryExecutor) Provider() provider.Provider {
	return q.provider
}

func getNodeResources(n *NodePattern, q *QueryExecutor, extraFilters []*KeyValuePair) (err error) {
	namespace := Namespace

	// Create a copy of ResourceProperties
	resourcePropertiesCopy := &ResourceProperties{}
	if n.ResourceProperties.Properties != nil {
		resourcePropertiesCopy.Properties = &Properties{
			PropertyList: make([]*Property, len(n.ResourceProperties.Properties.PropertyList)),
		}
		copy(resourcePropertiesCopy.Properties.PropertyList, n.ResourceProperties.Properties.PropertyList)
	}

	if resourcePropertiesCopy.Properties != nil && len(resourcePropertiesCopy.Properties.PropertyList) > 0 {
		for i, prop := range resourcePropertiesCopy.Properties.PropertyList {
			if prop.Key == "namespace" || prop.Key == "metadata.namespace" {
				namespace = prop.Value.(string)
				// Remove the namespace slice from the properties
				resourcePropertiesCopy.Properties.PropertyList = append(resourcePropertiesCopy.Properties.PropertyList[:i], resourcePropertiesCopy.Properties.PropertyList[i+1:]...)
			}
		}
	}

	var fieldSelector string
	var labelSelector string
	var hasNameSelector bool
	var hasLabelSelector bool

	if resourcePropertiesCopy.Properties != nil {
		for _, prop := range resourcePropertiesCopy.Properties.PropertyList {
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
	cacheKey := q.resourcePropertyName(n)
	if resultCache[cacheKey] == nil {
		// Get resources using the provider
		resources, err := q.provider.GetK8sResources(n.ResourceProperties.Kind, fieldSelector, labelSelector, namespace)
		if err != nil {
			return fmt.Errorf("error getting resources: %v", err)
		}

		// Apply extra filters from WHERE clause
		resourceList := resources.([]map[string]interface{})
		var filtered []map[string]interface{}

		// Process each resource
		for _, resource := range resourceList {
			keep := true
			// Apply extra filters
			for _, filter := range extraFilters {
				// Extract node name from filter key
				var resultMapKey string
				dotIndex := strings.Index(filter.Key, ".")
				if dotIndex != -1 {
					resultMapKey = filter.Key[:dotIndex]
				} else {
					resultMapKey = filter.Key
				}

				// Handle escaped dots
				for strings.HasSuffix(resultMapKey, "\\") {
					nextDotIndex := strings.Index(filter.Key[len(resultMapKey)+1:], ".")
					if nextDotIndex == -1 {
						resultMapKey = filter.Key
						break
					}
					resultMapKey = filter.Key[:len(resultMapKey)+1+nextDotIndex]
				}

				if resultMapKey == n.ResourceProperties.Name {
					// Transform path
					path := filter.Key
					path = strings.Replace(path, resultMapKey+".", "$.", 1)

					// Get value using jsonpath
					value, err := jsonpath.JsonPathLookup(resource, path)
					if err != nil {
						keep = false
						break
					}

					// Convert and compare values
					resourceValue, filterValue, err := convertToComparableTypes(value, filter.Value)
					if err != nil {
						keep = false
						break
					}

					// Compare based on operator
					switch filter.Operator {
					case "EQUALS", "=", "==":
						if resourceValue != filterValue {
							keep = false
						}
					case "GREATER_THAN", ">":
						if rv, ok := resourceValue.(float64); ok {
							if fv, ok := filterValue.(float64); ok {
								if rv <= fv {
									keep = false
								}
							}
						}
					case "LESS_THAN", "<":
						if rv, ok := resourceValue.(float64); ok {
							if fv, ok := filterValue.(float64); ok {
								if rv >= fv {
									keep = false
								}
							}
						}
					case "GREATER_THAN_EQUALS", ">=":
						if rv, ok := resourceValue.(float64); ok {
							if fv, ok := filterValue.(float64); ok {
								if rv < fv {
									keep = false
								}
							}
						}
					case "LESS_THAN_EQUALS", "<=":
						if rv, ok := resourceValue.(float64); ok {
							if fv, ok := filterValue.(float64); ok {
								if rv > fv {
									keep = false
								}
							}
						}
					case "NOT_EQUALS", "!=":
						if resourceValue == filterValue {
							keep = false
						}
					case "CONTAINS":
						strA := fmt.Sprintf("%v", resourceValue)
						strB := fmt.Sprintf("%v", filterValue)
						if !strings.Contains(strA, strB) {
							keep = false
						}
					case "REGEX_COMPARE":
						if filterValueStr, ok := filterValue.(string); ok {
							if resultValueStr, ok := resourceValue.(string); ok {
								if regex, err := regexp.Compile(filterValueStr); err == nil {
									if !regex.MatchString(resultValueStr) {
										keep = false
									}
								}
							}
						}
					}

					if !keep {
						break
					}
				}
			}

			if keep {
				filtered = append(filtered, resource)
			}
		}

		// Cache the filtered results
		resultCache[cacheKey] = filtered
		resultMap[n.ResourceProperties.Name] = filtered
	} else {
		resultMap[n.ResourceProperties.Name] = resultCache[cacheKey]
	}

	return nil
}

func GetContextQueryExecutor(context string) (*QueryExecutor, error) {
	executorsLock.RLock()
	if executor, exists := contextExecutors[context]; exists {
		executorsLock.RUnlock()
		return executor, nil
	}
	executorsLock.RUnlock()

	// Create new executor for this context
	executorsLock.Lock()
	defer executorsLock.Unlock()

	// Double-check after acquiring write lock
	if executor, exists := contextExecutors[context]; exists {
		return executor, nil
	}

	// Get the provider from the main executor instance
	if executorInstance == nil {
		return nil, fmt.Errorf("main executor instance not initialized")
	}

	executor, err := NewQueryExecutor(executorInstance.provider)
	if err != nil {
		return nil, fmt.Errorf("error creating query executor for context %s: %v", context, err)
	}

	if contextExecutors == nil {
		contextExecutors = make(map[string]*QueryExecutor)
	}
	contextExecutors[context] = executor
	return executor, nil
}

// Add these functions back
func InitGVRCache(p provider.Provider) error {
	if GvrCache == nil {
		GvrCache = make(map[string]schema.GroupVersionResource)
	}

	cache, err := p.GetGVRCache()
	if err != nil {
		return fmt.Errorf("error getting GVR cache: %w", err)
	}

	GvrCache = cache
	return nil
}

func InitResourceSpecs(p provider.Provider) error {
	if ResourceSpecs == nil {
		ResourceSpecs = make(map[string][]string)
	}

	specs, err := p.GetOpenAPIResourceSpecs()
	if err != nil {
		return fmt.Errorf("error getting resource specs: %w", err)
	}

	ResourceSpecs = specs

	// Initialize relationships after populating ResourceSpecs
	InitializeRelationships(ResourceSpecs)

	return nil
}

func extractKindFromSchemaName(schemaName string) string {
	parts := strings.Split(schemaName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}