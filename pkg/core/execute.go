package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/AvitalTamir/jsonpath"
)

func (q *QueryExecutor) ExecuteSingleQuery(ast *Expression, namespace string) (QueryResult, error) {
	if ast == nil {
		return QueryResult{}, fmt.Errorf("empty query: ast cannot be nil")
	}
	// Reset match nodes at the start of each query
	q.matchNodes = nil

	if AllNamespaces {
		Namespace = ""
		AllNamespaces = false // to reset value
	} else {
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
			// Store the nodes from the match clause
			q.matchNodes = c.Nodes

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
			err := q.handleSetClause(c)
			if err != nil {
				return *results, fmt.Errorf("error handling SET clause: %w", err)
			}

		case *DeleteClause:
			// Execute a Kubernetes delete operation based on the DeleteClause.
			for _, nodeId := range c.NodeIds {
				if resultMap[nodeId] == nil {
					// Skip error for expanded node identifiers
					if strings.Contains(nodeId, "__exp__") {
						debugLog("skipping error trying to delete expanded node identifier %s", nodeId)
						continue
					}
					return *results, fmt.Errorf("node identifier %s not found in result map", nodeId)
				}

				resources := resultMap[nodeId].([]map[string]interface{})
				for _, resource := range resources {
					metadata := resource["metadata"].(map[string]interface{})
					name := metadata["name"].(string)
					namespace := getNamespaceName(metadata)

					// Find the matching node from the stored match nodes
					var nodeKind string
					for _, node := range q.matchNodes {
						if node.ResourceProperties.Name == nodeId {
							nodeKind = node.ResourceProperties.Kind
							break
						}
					}
					if nodeKind == "" {
						return *results, fmt.Errorf("could not find kind for node %s in MATCH clause", nodeId)
					}

					err := q.provider.DeleteK8sResources(nodeKind, name, namespace)
					if err != nil {
						return *results, fmt.Errorf("error deleting resource %s/%s: %v", nodeKind, name, err)
					}
				}

				delete(resultMap, nodeId)
			}

		case *CreateClause:
			// Same as in Match clauses, we'll first look at relationships, then nodes
			// we'll iterates over the replationships then nodes, and from each we'll extract a spec and create the resource
			// in relationships, we'll need to find the matching node in the nodes list, we'll then construct the spec from the node properties and from the relevant part of the spec that's defined in the relationship
			// in nodes, we'll just construct the spec from the node properties

			// Iterate over the relationships in the create clause.
			// Process Relationships
			for _, rel := range c.Relationships {
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
				// Find the matching node from stored match nodes to get the kind
				var foreignNodeKind string
				for _, matchNode := range q.matchNodes {
					if matchNode.ResourceProperties.Name == foreignNode.ResourceProperties.Name {
						foreignNodeKind = matchNode.ResourceProperties.Kind
						break
					}
				}
				foreignGVR, err := q.findGVR(foreignNodeKind)
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
					return *results, fmt.Errorf("relationship rule not found for %s and %s - This code path should be invalid, likely problem with rule definitions", node.ResourceProperties.Kind, foreignNode.ResourceProperties.Kind)
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
				for idx, foreignResource := range resultMap[foreignNode.ResourceProperties.Name].([]map[string]interface{}) {
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
									currentForeignPart = currentForeignPart[part].([]interface{})[0].(map[string]interface{})
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
											currentPart = currentPart[part].([]interface{})[0].(map[string]interface{})
										} else {
											currentPart[part] = make(map[string]interface{})
											currentPart = currentPart[part].(map[string]interface{})
										}
									} else {
										if strings.HasSuffix(part, "[]") {
											part = strings.TrimSuffix(part, "[]")
											// if the part is an array, recurse into the first element
											currentPart = currentPart[part].([]interface{})[0].(map[string]interface{})
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

				// Transform path to jsonpath format
				path := strings.Replace(item.JsonPath, nodeId+".", "$.", 1)
				if path == "$." {
					path = "$"
				}

				// Compile and fix the path
				if !strings.Contains(path, ".") {
					path = "$"
				}
				compiledPath, err := jsonpath.Compile(path)
				if err != nil {
					return *results, fmt.Errorf("error compiling path %s: %v", path, err)
				}
				compiledPath = fixCompiledPath(compiledPath)

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

					result, err := compiledPath.Lookup(resource)
					if err != nil {
						debugLog("Path not found: %s", item.JsonPath)
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
								// Handle the case where the first result is a slice (from wildcard path)
								v := reflect.ValueOf(result)
								if v.Kind() == reflect.Slice {
									isCPUResource := strings.Contains(path, "resources.limits.cpu") || strings.Contains(path, "resources.requests.cpu")
									isMemoryResource := strings.Contains(path, "resources.limits.memory") || strings.Contains(path, "resources.requests.memory")

									isContainerContext := strings.Contains(path, "spec.containers")
									containsWildcard := strings.Contains(path, "[*]")

									if isContainerContext && containsWildcard {
										aggregateResult = result
									} else if isCPUResource {
										// Convert to string slice and sum
										cpuStrs, err := convertToStringSlice(v)
										if err != nil {
											return *results, fmt.Errorf("error converting CPU slice: %v", err)
										}

										cpuSum, err := sumMilliCPU(cpuStrs)
										if err != nil {
											return *results, err
										}
										aggregateResult = convertMilliCPUToStandard(cpuSum)
									} else if isMemoryResource {
										// Convert to string slice and sum
										memStrs, err := convertToStringSlice(v)
										if err != nil {
											return *results, fmt.Errorf("error converting memory slice: %v", err)
										}

										memSum, err := sumMemoryBytes(memStrs)
										if err != nil {
											return *results, err
										}
										aggregateResult = convertBytesToMemory(memSum)
									} else {
										// For other types, we can't sum slices
										return *results, fmt.Errorf("unsupported type for SUM: slice")
									}
								} else {
									aggregateResult = reflect.ValueOf(result).Interface()
								}
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

								isCPUResource := strings.Contains(path, "resources.limits.cpu") || strings.Contains(path, "resources.requests.cpu")
								isMemoryResource := strings.Contains(path, "resources.limits.memory") || strings.Contains(path, "resources.requests.memory")

								isContainerContext := strings.Contains(path, "spec.containers")
								containsWildcard := strings.Contains(path, "[*]")

								// Handle slice results (from wildcards)
								if v1.Kind() == reflect.Slice || v2.Kind() == reflect.Slice {
									if isContainerContext && containsWildcard {
										// For backward compatibility with tests, combine the slices
										if v1.Kind() == reflect.Slice && v2.Kind() == reflect.Slice {
											// Both are slices, combine them
											combined := reflect.MakeSlice(v1.Type(), 0, v1.Len()+v2.Len())
											for i := 0; i < v1.Len(); i++ {
												combined = reflect.Append(combined, v1.Index(i))
											}
											for i := 0; i < v2.Len(); i++ {
												combined = reflect.Append(combined, v2.Index(i))
											}
											aggregateResult = combined.Interface()
										} else if v1.Kind() == reflect.Slice {
											// v1 is a slice, v2 is not
											combined := reflect.MakeSlice(v1.Type(), 0, v1.Len()+1)
											for i := 0; i < v1.Len(); i++ {
												combined = reflect.Append(combined, v1.Index(i))
											}
											combined = reflect.Append(combined, v2)
											aggregateResult = combined.Interface()
										} else {
											// v2 is a slice, v1 is not
											combined := reflect.MakeSlice(v2.Type(), 0, v2.Len()+1)
											combined = reflect.Append(combined, v1)
											for i := 0; i < v2.Len(); i++ {
												combined = reflect.Append(combined, v2.Index(i))
											}
											aggregateResult = combined.Interface()
										}
									} else if isCPUResource {
										// Convert both values to string slices
										var cpuStrs []string

										// Handle if v1 is a slice
										if v1.Kind() == reflect.Slice {
											v1Strs, err := convertToStringSlice(v1)
											if err != nil {
												return *results, fmt.Errorf("error converting CPU slice: %v", err)
											}
											cpuStrs = append(cpuStrs, v1Strs...)
										} else {
											// v1 is a single value
											cpuStrs = append(cpuStrs, v1.String())
										}

										// Handle if v2 is a slice
										if v2.Kind() == reflect.Slice {
											v2Strs, err := convertToStringSlice(v2)
											if err != nil {
												return *results, fmt.Errorf("error converting CPU slice: %v", err)
											}
											cpuStrs = append(cpuStrs, v2Strs...)
										} else {
											// v2 is a single value
											cpuStrs = append(cpuStrs, v2.String())
										}

										// Sum all CPU values
										cpuSum, err := sumMilliCPU(cpuStrs)
										if err != nil {
											return *results, err
										}
										aggregateResult = convertMilliCPUToStandard(cpuSum)
									} else if isMemoryResource {
										// Convert both values to string slices
										var memStrs []string

										// Handle if v1 is a slice
										if v1.Kind() == reflect.Slice {
											v1Strs, err := convertToStringSlice(v1)
											if err != nil {
												return *results, fmt.Errorf("error converting memory slice: %v", err)
											}
											memStrs = append(memStrs, v1Strs...)
										} else {
											// v1 is a single value
											memStrs = append(memStrs, v1.String())
										}

										// Handle if v2 is a slice
										if v2.Kind() == reflect.Slice {
											v2Strs, err := convertToStringSlice(v2)
											if err != nil {
												return *results, fmt.Errorf("error converting memory slice: %v", err)
											}
											memStrs = append(memStrs, v2Strs...)
										} else {
											// v2 is a single value
											memStrs = append(memStrs, v2.String())
										}

										// Sum all memory values
										memSum, err := sumMemoryBytes(memStrs)
										if err != nil {
											return *results, err
										}
										aggregateResult = convertBytesToMemory(memSum)
									} else {
										// For other types, we can't sum slices
										return *results, fmt.Errorf("unsupported type for SUM: slice")
									}
								} else {
									switch v1.Kind() {
									case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
										aggregateResult = v1.Int() + v2.Int()
									case reflect.Float32, reflect.Float64:
										aggregateResult = v1.Float() + v2.Float()
									case reflect.String:
										if isCPUResource {
											// Convert CPU strings to millicores
											cpu1, err := convertToMilliCPU(v1.String())
											if err != nil {
												return *results, err
											}
											cpu2, err := convertToMilliCPU(v2.String())
											if err != nil {
												return *results, err
											}
											aggregateResult = convertMilliCPUToStandard(cpu1 + cpu2)
										} else if isMemoryResource {
											// Convert memory strings to bytes
											mem1, err := convertMemoryToBytes(v1.String())
											if err != nil {
												return *results, err
											}
											mem2, err := convertMemoryToBytes(v2.String())
											if err != nil {
												return *results, err
											}
											aggregateResult = convertBytesToMemory(mem1 + mem2)
										} else {
											// Handle unsupported types or error out
											return *results, fmt.Errorf("unsupported type for SUM: %v", v1.Kind())
										}
									default:
										// Handle unsupported types or error out
										return *results, fmt.Errorf("unsupported type for SUM: %v", v1.Kind())
									}
								}
							}
						}
					}

					if item.Aggregate == "" {
						key := item.Alias
						if key == "" {
							// Split the path into parts, excluding the node identifier
							path := strings.TrimPrefix(item.JsonPath, nodeId+".")

							// Split on unescaped dots only
							var pathParts []string
							var currentPart strings.Builder
							var escaped bool

							for i := 0; i < len(path); i++ {
								if escaped {
									// For escaped dots, add the dot without the backslash
									if path[i] == '.' {
										currentPart.WriteByte(path[i])
									} else {
										// For any other escaped character, keep both the backslash and the character
										currentPart.WriteByte('\\')
										currentPart.WriteByte(path[i])
									}
									escaped = false
									continue
								}

								if path[i] == '\\' {
									escaped = true
									continue
								}

								if path[i] == '.' && !escaped {
									if currentPart.Len() > 0 {
										pathParts = append(pathParts, currentPart.String())
										currentPart.Reset()
									}
								} else {
									currentPart.WriteByte(path[i])
								}
							}

							if currentPart.Len() > 0 {
								pathParts = append(pathParts, currentPart.String())
							}

							if len(pathParts) == 1 {
								key = pathParts[0]
							} else if len(pathParts) > 1 {
								// Restore nested structure
								nestedMap := currentMap
								for i := 0; i < len(pathParts)-1; i++ {
									if _, exists := nestedMap[pathParts[i]]; !exists {
										nestedMap[pathParts[i]] = make(map[string]interface{})
									}
									nestedMap = nestedMap[pathParts[i]].(map[string]interface{})
								}
								nestedMap[pathParts[len(pathParts)-1]] = result
								continue
							}
							if key == nodeId {
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
						key = strings.ToLower(item.Aggregate) + ":" + nodeId + "." + strings.TrimPrefix(path, "$.")
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
