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
	"time"

	"github.com/AvitalTamir/jsonpath"
	"github.com/avitaltamir/cyphernetes/pkg/provider"
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
var resultMapMutex sync.RWMutex

type QueryExecutor struct {
	provider       provider.Provider
	requestChannel chan *apiRequest
	semaphore      chan struct{}
	matchNodes     []*NodePattern
	currentAst     *Expression
}

var (
	Namespace     string
	LogLevel      string
	OutputFormat  string
	AllNamespaces bool
	CleanOutput   bool
	NoColor       bool
	// For testing
	mockFindPotentialKinds func([]*Relationship) []string
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
	if ast == nil {
		return QueryResult{}, fmt.Errorf("empty query: ast cannot be nil")
	}
	if len(ast.Contexts) > 0 {
		return ExecuteMultiContextQuery(ast, namespace)
	}

	// Store the current AST
	q.currentAst = ast

	// First, check for kindless nodes and rewrite the query if needed
	rewrittenAst, err := q.rewriteQueryForKindlessNodes(ast)
	if err != nil {
		return QueryResult{}, fmt.Errorf("error rewriting query: %w", err)
	}
	if rewrittenAst != nil {
		ast = rewrittenAst
		q.currentAst = rewrittenAst
	}

	result, err := q.ExecuteSingleQuery(ast, namespace)
	if err != nil {
		return result, err
	}

	// If this was a rewritten query, merge and deduplicate the results
	if rewrittenAst != nil {
		// Merge results with special pattern
		mergedResults := make(map[string]interface{})
		expResults := make(map[string][]interface{})
		aggregateResults := make(map[string]interface{})
		mergedGraph := Graph{
			Nodes: []Node{},
			Edges: []Edge{},
		}
		seenEdges := make(map[string]bool)

		// First pass: collect all expanded results
		for key, value := range result.Data {
			if key == "aggregate" {
				// Handle aggregate results separately
				if aggMap, ok := value.(map[string]interface{}); ok {
					for aggKey, aggValue := range aggMap {
						// Check if this is an expanded aggregate result
						if strings.HasPrefix(aggKey, "__exp__") {
							// Parse the expanded aggregate key: __exp__<type>__<name>__<index>
							parts := strings.Split(aggKey, "__")
							if len(parts) >= 5 {
								aggType := parts[2] // sum, count, etc.
								aggName := parts[3] // original name or alias

								// Initialize if not exists
								if _, exists := aggregateResults[aggName]; !exists {
									if aggType == "sum" {
										aggregateResults[aggName] = float64(0)
									} else if aggType == "count" {
										aggregateResults[aggName] = 0
									} else {
										aggregateResults[aggName] = make([]interface{}, 0)
									}
								}

								// Merge based on aggregate type
								switch aggType {
								case "sum":
									if aggValue != nil {
										currentSum := aggregateResults[aggName].(float64)
										switch v := aggValue.(type) {
										case float64:
											aggregateResults[aggName] = currentSum + v
										case int:
											aggregateResults[aggName] = currentSum + float64(v)
										case int64:
											aggregateResults[aggName] = currentSum + float64(v)
										}
									}
								case "count":
									if aggValue != nil {
										currentCount := aggregateResults[aggName].(int)
										if count, ok := aggValue.(int); ok {
											aggregateResults[aggName] = currentCount + count
										}
									}
								default:
									// For other aggregates, collect all non-nil values
									if aggValue != nil {
										arr := aggregateResults[aggName].([]interface{})
										aggregateResults[aggName] = append(arr, aggValue)
									}
								}
							}
						}
					}
				}
			} else if strings.Contains(key, "__exp__") {
				// Extract original variable name (everything before __exp__)
				origVar := strings.Split(key, "__exp__")[0]
				if expResults[origVar] == nil {
					expResults[origVar] = make([]interface{}, 0)
				}
				if valueSlice, ok := value.([]interface{}); ok && len(valueSlice) > 0 {
					expResults[origVar] = append(expResults[origVar], valueSlice...)
				}
			} else {
				mergedResults[key] = value
			}
		}

		// Add aggregated results back to merged results
		if len(aggregateResults) > 0 {
			mergedResults["aggregate"] = aggregateResults
		}

		// Clean up node IDs and add to merged graph
		for _, node := range result.Graph.Nodes {
			if strings.Contains(node.Id, "__exp__") {
				node.Id = strings.Split(node.Id, "__exp__")[0]
			}
			mergedGraph.Nodes = append(mergedGraph.Nodes, node)
		}

		// Clean up and deduplicate edges
		for _, edge := range result.Graph.Edges {
			if strings.Contains(edge.From, "__exp__") {
				edge.From = strings.Split(edge.From, "__exp__")[0]
			}
			if strings.Contains(edge.To, "__exp__") {
				edge.To = strings.Split(edge.To, "__exp__")[0]
			}
			edgeKey := fmt.Sprintf("%s-%s-%s", edge.From, edge.To, edge.Type)
			if !seenEdges[edgeKey] {
				seenEdges[edgeKey] = true
				mergedGraph.Edges = append(mergedGraph.Edges, edge)
			}
		}

		// Second pass: merge expanded results and deduplicate
		for origVar, values := range expResults {
			if len(values) > 0 {
				// Deduplicate values
				seen := make(map[string]interface{})
				deduped := make([]interface{}, 0)

				for _, val := range values {
					// Convert value to string for comparison
					valBytes, err := json.Marshal(val)
					if err != nil {
						continue
					}
					valStr := string(valBytes)

					if _, exists := seen[valStr]; !exists {
						seen[valStr] = val
						deduped = append(deduped, val)
					}
				}

				mergedResults[origVar] = deduped
			}
		}

		result.Data = mergedResults
		result.Graph = mergedGraph
	}

	return result, nil
}

func (q *QueryExecutor) ExecuteSingleQuery(ast *Expression, namespace string) (QueryResult, error) {
	if ast == nil {
		return QueryResult{}, fmt.Errorf("empty query: ast cannot be nil")
	}
	// Reset match nodes at the start of each query
	q.matchNodes = nil

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

								isCPUResource := strings.Contains(path, "resources.limits.cpu") || strings.Contains(path, "resources.requests.cpu")
								isMemoryResource := strings.Contains(path, "resources.limits.memory") || strings.Contains(path, "resources.requests.memory")

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

func (q *QueryExecutor) rewriteQueryForKindlessNodes(expr *Expression) (*Expression, error) {
	// Find all kindless nodes and their relationships
	var kindlessNodes []*NodePattern
	var relationships []*Relationship

	for _, c := range expr.Clauses {
		if matchClause, ok := c.(*MatchClause); ok {
			// Find kindless nodes
			for _, node := range matchClause.Nodes {
				if node.ResourceProperties.Kind == "" {
					kindlessNodes = append(kindlessNodes, node)
				}
			}

			// Collect relationships
			relationships = append(relationships, matchClause.Relationships...)

			// Check for standalone kindless nodes
			for _, node := range kindlessNodes {
				isInRelationship := false
				for _, rel := range matchClause.Relationships {
					if rel.LeftNode.ResourceProperties.Name == node.ResourceProperties.Name ||
						rel.RightNode.ResourceProperties.Name == node.ResourceProperties.Name {
						isInRelationship = true
						break
					}
				}
				if !isInRelationship {
					return nil, fmt.Errorf("kindless nodes may only be used in a relationship")
				}
			}

			// Check for kindless-to-kindless chains in relationships
			for _, rel := range matchClause.Relationships {
				if rel.LeftNode.ResourceProperties.Kind == "" && rel.RightNode.ResourceProperties.Kind == "" {
					return nil, fmt.Errorf("chaining two unknown nodes (kindless-to-kindless) is not supported - at least one node in a relationship must have a known kind")
				}
			}
		}
	}

	// If no kindless nodes, no rewrite needed
	if len(kindlessNodes) == 0 {
		return nil, nil
	}

	// Find potential kinds for each kindless node
	var potentialKinds []string
	var err error
	if mockFindPotentialKinds != nil {
		// Use mock function in tests
		potentialKinds = mockFindPotentialKinds(relationships)
	} else {
		// Use real function in production
		potentialKinds, err = FindPotentialKindsIntersection(relationships, q.provider)
		if err != nil {
			return nil, fmt.Errorf("unable to determine kind for nodes in relationship >> %s", err)
		}
	}

	if len(potentialKinds) == 0 {
		return nil, fmt.Errorf("unable to determine kind for nodes in relationship")
	}

	// Build expanded query
	var matchParts []string
	var returnParts []string
	var setParts []string
	var deleteParts []string
	var whereParts []string
	var seenNodes = make(map[string]bool)

	// For each potential kind, create a match pattern and return item
	for i, kind := range potentialKinds {
		for _, clause := range expr.Clauses {
			switch c := clause.(type) {
			case *MatchClause:
				// Build match pattern
				var nodeParts []string
				for _, rel := range c.Relationships {
					var leftNodeStr, rightNodeStr string

					// Build left node pattern
					if rel.LeftNode.ResourceProperties.Kind == "" {
						leftNodeStr = fmt.Sprintf("(%s__exp__%d:%s", rel.LeftNode.ResourceProperties.Name, i, kind)
					} else {
						leftNodeStr = fmt.Sprintf("(%s__exp__%d:%s", rel.LeftNode.ResourceProperties.Name, i, rel.LeftNode.ResourceProperties.Kind)
					}

					// Add properties to left node if any
					if rel.LeftNode.ResourceProperties.Properties != nil {
						props := make([]string, 0)
						for _, prop := range rel.LeftNode.ResourceProperties.Properties.PropertyList {
							var valueStr string
							switch v := prop.Value.(type) {
							case string:
								valueStr = fmt.Sprintf("\"%s\"", v)
							default:
								valueStr = fmt.Sprintf("%v", v)
							}
							props = append(props, fmt.Sprintf("%s: %s", prop.Key, valueStr))
						}
						leftNodeStr += fmt.Sprintf(" {%s}", strings.Join(props, ", "))
					}
					leftNodeStr += ")"

					// Build right node pattern
					if rel.RightNode.ResourceProperties.Kind == "" {
						rightNodeStr = fmt.Sprintf("(%s__exp__%d:%s", rel.RightNode.ResourceProperties.Name, i, kind)
					} else {
						rightNodeStr = fmt.Sprintf("(%s__exp__%d:%s", rel.RightNode.ResourceProperties.Name, i, rel.RightNode.ResourceProperties.Kind)
					}

					// Add properties to right node if any
					if rel.RightNode.ResourceProperties.Properties != nil {
						props := make([]string, 0)
						for _, prop := range rel.RightNode.ResourceProperties.Properties.PropertyList {
							var valueStr string
							switch v := prop.Value.(type) {
							case string:
								valueStr = fmt.Sprintf("\"%s\"", v)
							default:
								valueStr = fmt.Sprintf("%v", v)
							}
							props = append(props, fmt.Sprintf("%s: %s", prop.Key, valueStr))
						}
						rightNodeStr += fmt.Sprintf(" {%s}", strings.Join(props, ", "))
					}
					rightNodeStr += ")"

					// Add relationship pattern
					nodeParts = append(nodeParts, fmt.Sprintf("%s->%s", leftNodeStr, rightNodeStr))
				}
				if len(nodeParts) > 0 {
					matchParts = append(matchParts, strings.Join(nodeParts, ", "))
				}

				// Handle WHERE conditions from ExtraFilters
				for _, extraFilter := range c.ExtraFilters {
					if extraFilter.Type == "KeyValuePair" {
						filter := extraFilter.KeyValuePair
						parts := strings.Split(filter.Key, ".")
						if len(parts) > 0 {
							nodeName := parts[0]
							propertyPath := strings.Join(parts[1:], ".")

							for j := 0; j < len(potentialKinds); j++ {
								varName := fmt.Sprintf("%s__exp__%d", nodeName, j)
								var valueStr string
								switch v := filter.Value.(type) {
								case string:
									valueStr = fmt.Sprintf("\"%s\"", v)
								default:
									valueStr = fmt.Sprintf("%v", v)
								}
								// Map operator names to symbols
								operator := filter.Operator
								switch operator {
								case "EQUALS":
									operator = "="
								case "NOT_EQUALS":
									operator = "!="
								case "GREATER_THAN":
									operator = ">"
								case "LESS_THAN":
									operator = "<"
								case "GREATER_THAN_EQUALS":
									operator = ">="
								case "LESS_THAN_EQUALS":
									operator = "<="
								case "REGEX_COMPARE":
									operator = "=~"
								case "CONTAINS":
									operator = "CONTAINS"
								case "":
									operator = "="
								}

								notPrefix := ""
								if filter.IsNegated {
									notPrefix = "NOT "
								}
								whereParts = append(whereParts, fmt.Sprintf("%s%s.%s %s %s", notPrefix, varName, propertyPath, operator, valueStr))
							}
						}
					}
				}

			case *ReturnClause:
				// Build return items
				for _, item := range c.Items {
					if !seenNodes[item.JsonPath] {
						seenNodes[item.JsonPath] = true
						// Add a return item for each iteration
						for j := 0; j < len(potentialKinds); j++ {
							var returnItem string
							if strings.Contains(item.JsonPath, ".") {
								parts := strings.SplitN(item.JsonPath, ".", 2)
								varName := fmt.Sprintf("%s__exp__%d", parts[0], j)
								returnPath := fmt.Sprintf("%s.%s", varName, parts[1])
								if item.Aggregate != "" {
									returnItem = fmt.Sprintf("%s {%s}", item.Aggregate, returnPath)
								} else {
									returnItem = returnPath
								}
							} else {
								varName := fmt.Sprintf("%s__exp__%d", item.JsonPath, j)
								if item.Aggregate != "" {
									returnItem = fmt.Sprintf("%s {%s}", item.Aggregate, varName)
								} else {
									returnItem = varName
								}
							}
							// Add AS alias with expansion pattern
							if item.Aggregate != "" {
								aggType := strings.ToLower(item.Aggregate)
								if item.Alias != "" {
									returnItem = fmt.Sprintf("%s AS __exp__%s__%s__%d", returnItem, aggType, item.Alias, j)
								} else {
									// For non-aliased aggregations, use the format <aggregate_type>_<node>_<path>
									aliasPath := item.JsonPath
									aliasPath = strings.Replace(aliasPath, ".", "_", -1)
									returnItem = fmt.Sprintf("%s AS __exp__%s__%s_%s__%d", returnItem, aggType, aggType, aliasPath, j)
								}
							} else if item.Alias != "" {
								returnItem = fmt.Sprintf("%s AS %s", returnItem, item.Alias)
							}
							returnParts = append(returnParts, returnItem)
						}
					}
				}
			case *SetClause:
				// Build set items
				for _, kvp := range c.KeyValuePairs {
					if strings.Contains(kvp.Key, ".") {
						parts := strings.SplitN(kvp.Key, ".", 2)
						// If the node being set is kindless, we need to create a set clause for each potential kind
						if isKindless(parts[0], kindlessNodes) {
							for j := 0; j < len(potentialKinds); j++ {
								varName := fmt.Sprintf("%s__exp__%d", parts[0], j)
								setPath := fmt.Sprintf("%s.%s", varName, parts[1])
								var valueStr string
								switch v := kvp.Value.(type) {
								case string:
									valueStr = fmt.Sprintf("\"%s\"", v)
								default:
									valueStr = fmt.Sprintf("%v", v)
								}
								setParts = append(setParts, fmt.Sprintf("%s = %s", setPath, valueStr))
							}
						} else {
							// If the node is not kindless, just use it as is
							varName := fmt.Sprintf("%s__exp__0", parts[0])
							setPath := fmt.Sprintf("%s.%s", varName, parts[1])
							var valueStr string
							switch v := kvp.Value.(type) {
							case string:
								valueStr = fmt.Sprintf("'%s'", v)
							default:
								valueStr = fmt.Sprintf("%v", v)
							}
							setParts = append(setParts, fmt.Sprintf("%s = %s", setPath, valueStr))
						}
					}
				}
			case *DeleteClause:
				// Build delete items
				for _, nodeId := range c.NodeIds {
					// If the node being deleted is kindless, we need to create a delete clause for each potential kind
					if isKindless(nodeId, kindlessNodes) {
						for j := 0; j < len(potentialKinds); j++ {
							varName := fmt.Sprintf("%s__exp__%d", nodeId, j)
							deleteParts = append(deleteParts, varName)
						}
					} else {
						// If the node is not kindless, just use it as is
						varName := fmt.Sprintf("%s__exp__0", nodeId)
						deleteParts = append(deleteParts, varName)
					}
				}
			}
		}
	}

	// Combine into final query
	var queryParts []string
	if len(matchParts) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("MATCH %s", strings.Join(matchParts, ", ")))
	}
	if len(whereParts) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("WHERE %s", strings.Join(whereParts, ", ")))
	}
	if len(setParts) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("SET %s", strings.Join(setParts, ", ")))
	}
	if len(deleteParts) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("DELETE %s", strings.Join(deleteParts, ", ")))
	}
	if len(returnParts) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("RETURN %s", strings.Join(returnParts, ", ")))
	}

	query := strings.Join(queryParts, " ")

	// Log the expanded query for debugging
	logDebug(fmt.Sprintf("Expanded query: %s\n", query))

	// Parse the expanded query into a new AST
	newAst, err := ParseQuery(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing expanded query: %w", err)
	}

	return newAst, nil
}

// QueryExpandedError is a special error type that indicates the query was expanded
type QueryExpandedError struct {
	ExpandedQuery string
}

func (e *QueryExpandedError) Error() string {
	return fmt.Sprintf("query expanded to: %s", e.ExpandedQuery)
}

func (q *QueryExecutor) processRelationship(rel *Relationship, c *MatchClause, results *QueryResult, filteredResults map[string][]map[string]interface{}) (bool, error) {
	logDebug(fmt.Sprintf("Processing relationship: %+v\n", rel))

	// Determine relationship type and fetch related resources
	var relType RelationshipType

	// Resolve kinds if needed
	if rel.LeftNode.ResourceProperties.Kind == "" || rel.RightNode.ResourceProperties.Kind == "" {
		// Try to resolve the kind using relationships
		potentialKinds, err := FindPotentialKindsIntersection(c.Relationships, q.provider)
		if err != nil {
			return false, fmt.Errorf("unable to determine kind for nodes in relationship >> %s", err)
		}
		if len(potentialKinds) == 0 {
			return false, fmt.Errorf("unable to determine kind for nodes in relationship")
		}
		if len(potentialKinds) > 1 {
			// Instead of expanding the query here, we'll let rewriteQueryForKindlessNodes handle it
			return false, &QueryExpandedError{ExpandedQuery: "needs_rewrite"}
		}
		if rel.LeftNode.ResourceProperties.Kind == "" {
			rel.LeftNode.ResourceProperties.Kind = potentialKinds[0]
		}
		if rel.RightNode.ResourceProperties.Kind == "" {
			rel.RightNode.ResourceProperties.Kind = potentialKinds[0]
		}
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

	resultMapMutex.RLock()
	if rule.KindA == rightKind.Resource {
		resourcesA = getResourcesFromMap(filteredResults, rel.RightNode.ResourceProperties.Name)
		resourcesB = getResourcesFromMap(filteredResults, rel.LeftNode.ResourceProperties.Name)
		filteredDirection = Left
	} else if rule.KindA == leftKind.Resource {
		resourcesA = getResourcesFromMap(filteredResults, rel.LeftNode.ResourceProperties.Name)
		resourcesB = getResourcesFromMap(filteredResults, rel.RightNode.ResourceProperties.Name)
		filteredDirection = Right
	} else {
		resultMapMutex.RUnlock()
		return false, fmt.Errorf("relationship rule not found for %s and %s", rel.LeftNode.ResourceProperties.Kind, rel.RightNode.ResourceProperties.Kind)
	}
	resultMapMutex.RUnlock()

	matchedResources := applyRelationshipRule(resourcesA, resourcesB, rule, filteredDirection)

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

	// Process edges
	for _, rightResource := range rightResources {
		for _, leftResource := range leftResources {
			// Check if these resources actually match according to the criteria
			for _, criterion := range rule.MatchCriteria {
				if matchByCriterion(rightResource, leftResource, criterion) || matchByCriterion(leftResource, rightResource, criterion) {
					rightNodeId := fmt.Sprintf("%s/%s", rightResource["kind"].(string), rightResource["metadata"].(map[string]interface{})["name"].(string))
					leftNodeId := fmt.Sprintf("%s/%s", leftResource["kind"].(string), leftResource["metadata"].(map[string]interface{})["name"].(string))
					results.Graph.Edges = append(results.Graph.Edges, Edge{
						From: rightNodeId,
						To:   leftNodeId,
						Type: string(relType),
					})
				}
			}
		}
	}

	filteredA := len(matchedResources["right"].([]map[string]interface{})) < len(resourcesA)
	filteredB := len(matchedResources["left"].([]map[string]interface{})) < len(resourcesB)

	filteredResults[rel.RightNode.ResourceProperties.Name] = matchedResources["right"].([]map[string]interface{})
	filteredResults[rel.LeftNode.ResourceProperties.Name] = matchedResources["left"].([]map[string]interface{})

	resultMapMutex.Lock()
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
	resultMapMutex.Unlock()

	return filteredA || filteredB, nil
}

func getResourcesFromMap(filteredResults map[string][]map[string]interface{}, key string) []map[string]interface{} {
	if filtered, ok := filteredResults[key]; ok {
		return filtered
	}

	resultMapMutex.RLock()
	defer resultMapMutex.RUnlock()

	if resources, ok := resultMap[key].([]map[string]interface{}); ok {
		return resources
	}
	return nil
}

func (q *QueryExecutor) processNodes(c *MatchClause, results *QueryResult) error {
	logDebug(fmt.Sprintf("Processing nodes, current graph nodes: %+v\n", results.Graph.Nodes))

	// Track seen nodes for deduplication
	seenNodes := make(map[string]bool)
	for _, existingNode := range results.Graph.Nodes {
		nodeKey := fmt.Sprintf("%s/%s", existingNode.Kind, existingNode.Name)
		seenNodes[nodeKey] = true
	}

	for _, node := range c.Nodes {
		if node.ResourceProperties.Kind == "" {
			// Try to resolve the kind using relationships
			potentialKinds, err := FindPotentialKindsIntersection(c.Relationships, q.provider)
			if err != nil {
				return fmt.Errorf("unable to determine kind for node '%s' - no relationships found", node.ResourceProperties.Name)
			}
			if len(potentialKinds) == 0 {
				return fmt.Errorf("unable to determine kind for node '%s' - no relationships found", node.ResourceProperties.Name)
			}
			if len(potentialKinds) > 1 {
				return fmt.Errorf("ambiguous kind for node '%s' - possible kinds: %v", node.ResourceProperties.Name, potentialKinds)
			}
			node.ResourceProperties.Kind = potentialKinds[0]
		}

		// check if the node has already been fetched
		cacheKey, err := q.resourcePropertyName(node)
		if err != nil {
			return fmt.Errorf("error getting resource property name: %v", err)
		}
		if resultCache[cacheKey] == nil {
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
				newNode := Node{
					Id:   node.ResourceProperties.Name,
					Kind: resource["kind"].(string),
					Name: metadata["name"].(string),
				}
				if newNode.Kind != "Namespace" {
					newNode.Namespace = getNamespaceName(metadata)
				}

				// Check if we've seen this node before
				nodeKey := fmt.Sprintf("%s/%s", newNode.Kind, newNode.Name)
				if !seenNodes[nodeKey] {
					seenNodes[nodeKey] = true
					logDebug(fmt.Sprintf("Adding new unique node from processNodes: %+v with key: %s\n", newNode, nodeKey))
					results.Graph.Nodes = append(results.Graph.Nodes, newNode)
				} else {
					logDebug(fmt.Sprintf("Skipping duplicate node in processNodes: %+v with key: %s\n", newNode, nodeKey))
				}
			}
		} else if resultMap[node.ResourceProperties.Name] == nil {
			// Copy from cache using the original name
			resultMap[node.ResourceProperties.Name] = resultCache[cacheKey]
		}
	}
	logDebug(fmt.Sprintf("After processNodes, graph nodes: %+v\n", results.Graph.Nodes))
	return nil
}

func (q *QueryExecutor) buildGraph(result *QueryResult) {
	logDebug(fmt.Sprintln("Building graph"))
	logDebug(fmt.Sprintf("Initial nodes: %+v\n", result.Graph.Nodes))

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
			logDebug(fmt.Sprintf("Adding node from result data: %+v\n", node))
			result.Graph.Nodes = append(result.Graph.Nodes, node)
		}
	}

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

func (q *QueryExecutor) resourcePropertyName(n *NodePattern) (string, error) {
	var ns string

	gvr, err := q.provider.FindGVR(n.ResourceProperties.Kind)
	if err != nil {
		return "", err
	}

	if n.ResourceProperties.Properties == nil {
		return fmt.Sprintf("%s_%s", Namespace, gvr.Resource), nil
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

	return fmt.Sprintf("%s_%s", ns, gvr.Resource), nil
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
	fmt.Printf("Creating patch for path: %v with value: %v\n", path, value)

	// For any path that ends with a key that might contain dots (like labels or annotations),
	// we need to handle it specially to ensure we only update that specific key
	if len(path) >= 2 {
		// Check if we're updating a map field (like labels, annotations, or any other map)
		// We'll detect this by checking if the last part of the path contains dots or slashes
		// which would indicate it's a key in a map
		lastPart := path[len(path)-1]
		if strings.Contains(lastPart, ".") || strings.Contains(lastPart, "/") {
			fmt.Printf("Detected path with dots in last part: %s\n", lastPart)
			// This is likely a key in a map, so we should create a patch that only updates this key
			// Extract the map path (everything except the last part)
			mapPath := path[:len(path)-1]
			jsonMapPath := "/" + strings.Join(mapPath, "/")

			// For array indices, convert [n] to /n
			re := regexp.MustCompile(`\[(\d+)\]`)
			jsonMapPath = re.ReplaceAllString(jsonMapPath, "/$1")

			fmt.Printf("Map path: %s\n", jsonMapPath)

			// Create a patch that uses the "test" operation to check if the map exists
			// If it doesn't, this will fail and we'll need to create it
			testPatch := map[string]interface{}{
				"op":    "test",
				"path":  jsonMapPath,
				"value": map[string]interface{}{},
			}

			// Create a patch that adds just the specific key-value pair to the map
			// Properly escape the key according to JSON Patch spec:
			// '~' is escaped as '~0' and '/' is escaped as '~1'
			escapedKey := strings.ReplaceAll(lastPart, "~", "~0")
			escapedKey = strings.ReplaceAll(escapedKey, "/", "~1")
			// Do NOT escape dots - they are valid in JSON Patch paths
			fmt.Printf("Escaped key: %s\n", escapedKey)

			addPatch := map[string]interface{}{
				"op":    "add",
				"path":  jsonMapPath + "/" + escapedKey,
				"value": value,
			}

			fmt.Printf("Created test patch: %+v\n", testPatch)
			fmt.Printf("Created add patch: %+v\n", addPatch)

			return []interface{}{testPatch, addPatch}
		}
	}

	// For regular paths, we need to ensure all parent paths exist
	// Special handling for common paths that need to be created as empty objects
	// This is a more targeted approach than trying to build every possible path
	if len(path) >= 2 {
		// Check for common paths that might need to be created first
		if path[0] == "metadata" {
			if len(path) >= 3 && (path[1] == "annotations" || path[1] == "labels") {
				// For metadata.labels or metadata.annotations, we need to use a test patch
				// followed by an add patch to trigger the strategic merge patch in the provider
				mapPath := fmt.Sprintf("/metadata/%s", path[1])

				// Create a test patch to check if the map exists
				testPatch := map[string]interface{}{
					"op":    "test",
					"path":  mapPath,
					"value": map[string]interface{}{},
				}

				// Create an add patch for the specific key
				keyPath := fmt.Sprintf("%s/%s", mapPath, path[2])
				addPatch := map[string]interface{}{
					"op":    "add",
					"path":  keyPath,
					"value": value,
				}

				fmt.Printf("Created test patch: %+v\n", testPatch)
				fmt.Printf("Created add patch: %+v\n", addPatch)

				return []interface{}{testPatch, addPatch}
			}
		} else if path[0] == "spec" && len(path) >= 3 {
			if path[1] == "template" && path[2] == "spec" {
				// Handle pod spec paths like spec.template.spec.containers[0].resources
				if len(path) >= 5 && strings.HasPrefix(path[3], "containers[") {
					// Extract the container index
					containerIndexMatch := regexp.MustCompile(`\[(\d+)\]`).FindStringSubmatch(path[3])
					if len(containerIndexMatch) < 2 {
						// If we can't extract the index, fall back to regular patching
						goto DEFAULT_CASE
					}

					containerIndex := containerIndexMatch[1]

					// For container resources, we'll use a special approach
					// Create a single strategic merge patch instead of multiple JSON patches
					// This is more reliable for container resources

					// Create a special patch that signals to the provider to use a strategic merge patch
					specialPatch := map[string]interface{}{
						"op":    "test",
						"path":  "/spec/template/spec/containers",
						"value": []interface{}{},
					}

					// Create a patch that adds the specific property
					// We'll use a special format that the provider will recognize
					// and convert to a strategic merge patch
					containerPath := fmt.Sprintf("/spec/template/spec/containers/%s", containerIndex)

					// Build the path to the specific property
					propertyPath := containerPath
					for i := 4; i < len(path); i++ {
						part := path[i]
						// Handle array indices in the path
						if strings.Contains(part, "[") && strings.Contains(part, "]") {
							part = regexp.MustCompile(`\[(\d+)\]`).ReplaceAllString(part, "/$1")
						}
						propertyPath += "/" + part
					}

					// Create the add patch
					addPatch := map[string]interface{}{
						"op":    "add",
						"path":  propertyPath,
						"value": value,
					}

					fmt.Printf("Created special container patch: %+v\n", specialPatch)
					fmt.Printf("Created add patch: %+v\n", addPatch)

					return []interface{}{specialPatch, addPatch}
				}
			}
		}
	}

DEFAULT_CASE:
	// For all other paths, use a simple add operation
	// This will work if the parent path already exists
	jsonPath := "/" + strings.Join(path, "/")

	// For array indices, convert [n] to /n
	re := regexp.MustCompile(`\[(\d+)\]`)
	jsonPath = re.ReplaceAllString(jsonPath, "/$1")

	fmt.Printf("Regular path: %s\n", jsonPath)

	patch := map[string]interface{}{
		"op":    "add",
		"path":  jsonPath,
		"value": value,
	}

	fmt.Printf("Created regular patch: %+v\n", patch)

	return []interface{}{patch}
}

func setValueAtPath(data interface{}, path string, value interface{}) error {
	// Convert path to array of parts
	parts := strings.Split(strings.TrimPrefix(path, "."), ".")

	// Create compatible patch format
	patches := createCompatiblePatch(parts, value)

	// Apply patches to the data
	if m, ok := data.(map[string]interface{}); ok {
		// First update the in-memory representation
		updateResultMap(m, parts, value)

		// Then apply the JSON patch if needed
		patchJSON, err := json.Marshal(patches)
		if err != nil {
			return fmt.Errorf("error marshalling patches: %s", err)
		}

		// Store the patch for later use if needed
		if metadata, ok := m["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				logDebug("Created patch for %s: %s", name, string(patchJSON))
			}
		}

		return nil
	}

	return fmt.Errorf("data must be a map[string]interface{}, got %T", data)
}

// Move updateResultMap to be near setValueAtPath for better code organization
func updateResultMap(resource map[string]interface{}, path []string, value interface{}) {
	current := resource
	for i := 0; i < len(path)-1; i++ {
		part := path[i]
		if current[part] == nil {
			current[part] = make(map[string]interface{})
		}
		if m, ok := current[part].(map[string]interface{}); ok {
			current = m
		} else {
			// If it's not a map, create one
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}
	current[path[len(path)-1]] = value
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
			return fmt.Sprintf("1%s", unit.suffix)
		}
	}

	// Then check for binary units (power-of-two)
	for _, unit := range binaryUnits {
		if bytes >= unit.multiplier {
			value := float64(bytes) / float64(unit.multiplier)
			formatted := strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
			return fmt.Sprintf("%s%s", formatted, unit.suffix)
		}
	}

	// If no binary unit applies, check for decimal units (power-of-ten)
	for _, unit := range decimalUnits {
		if bytes >= unit.multiplier {
			value := float64(bytes) / float64(unit.multiplier)
			formatted := strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
			return fmt.Sprintf("%s%s", formatted, unit.suffix)
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

func ExecuteMultiContextQuery(ast *Expression, namespace string) (QueryResult, error) {
	if len(ast.Contexts) == 0 {
		return QueryResult{}, fmt.Errorf("no contexts provided for multi-context query")
	}

	// Initialize combined results
	combinedResults := QueryResult{
		Data: make(map[string]interface{}),
		Graph: Graph{
			Nodes: []Node{},
			Edges: []Edge{},
		},
	}

	// Execute query for each context
	for _, context := range ast.Contexts {
		executor, err := GetContextQueryExecutor(context)
		if err != nil {
			return combinedResults, fmt.Errorf("error getting executor for context %s: %v", context, err)
		}

		// Create a modified AST with prefixed variables
		modifiedAst := prefixVariables(ast, context)

		// Use ExecuteSingleQuery instead of Execute
		result, err := executor.ExecuteSingleQuery(modifiedAst, namespace)
		if err != nil {
			return combinedResults, fmt.Errorf("error executing query in context %s: %v", context, err)
		}

		// Merge results
		for k, v := range result.Data {
			combinedResults.Data[k] = v
		}
		combinedResults.Graph.Nodes = append(combinedResults.Graph.Nodes, result.Graph.Nodes...)
		combinedResults.Graph.Edges = append(combinedResults.Graph.Edges, result.Graph.Edges...)
	}

	return combinedResults, nil
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
		InitializeRelationships(ResourceSpecs, p)
	})
	return executorInstance
}

func (q *QueryExecutor) Provider() provider.Provider {
	return q.provider
}

func fixCompiledPath(compiledPath *jsonpath.Compiled) *jsonpath.Compiled {
	i := 0
	for i < len(compiledPath.Steps) {
		step := compiledPath.Steps[i]
		if strings.HasSuffix(step.Key, "\\") && i+1 < len(compiledPath.Steps) {
			nextStep := compiledPath.Steps[i+1]
			step.Key = step.Key[:len(step.Key)-1] + "." + nextStep.Key
			compiledPath.Steps[i] = step
			compiledPath.Steps = append(compiledPath.Steps[:i+1], compiledPath.Steps[i+2:]...)
		} else {
			i++
		}
	}
	return compiledPath
}

func getNodeResources(n *NodePattern, q *QueryExecutor, extraFilters []*Filter) (err error) {
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
	cacheKey, err := q.resourcePropertyName(n)
	if err != nil {
		return fmt.Errorf("error getting resource property name: %v", err)
	}

	// Lock for reading from cache
	resultMapMutex.RLock()
	cachedResult := resultCache[cacheKey]
	resultMapMutex.RUnlock()

	if cachedResult == nil {
		// Get resources using the provider
		resources, err := q.provider.GetK8sResources(n.ResourceProperties.Kind, fieldSelector, labelSelector, namespace)
		if err != nil {
			return fmt.Errorf("error getting resources: %v", err)
		}

		// Apply extra filters from WHERE clause
		resourceList := resources.([]map[string]interface{})
		var filtered []map[string]interface{}

		var subMatches []*SubMatch

		// Process each resource
		for _, resource := range resourceList {
			keep := true
			// Apply extra filters
			for _, extraFilter := range extraFilters {
				if extraFilter.Type == "KeyValuePair" {
					filter := extraFilter.KeyValuePair
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

						// Compile and fix the path
						compiledPath, err := jsonpath.Compile(path)
						if err != nil {
							keep = false
							break
						}
						compiledPath = fixCompiledPath(compiledPath)

						logDebug("Looking up path: %s in resource: %+v", path, resource)

						// If path contains wildcards, we need special handling
						if strings.Contains(path, "[*]") {
							keep = evaluateWildcardPath(resource, path, filter.Value, filter.Operator)
							if filter.IsNegated {
								keep = !keep
							}
						} else {
							// Regular path handling using the fixed compiled path
							value, err := compiledPath.Lookup(resource)
							if err != nil {
								keep = false
								break
							}

							// Check if the filter value is a temporal expression
							if temporalExpr, ok := filter.Value.(*TemporalExpression); ok {
								// Convert resource value to time.Time if it's a string
								var resourceTime time.Time
								if timeStr, ok := value.(string); ok {
									resourceTime, err = time.Parse(time.RFC3339, timeStr)
									if err != nil {
										keep = false
										break
									}
								} else {
									keep = false
									break
								}

								// Use temporal handler to compare values
								temporalHandler := NewTemporalHandler()
								keep, err = temporalHandler.CompareTemporalValues(resourceTime, temporalExpr, filter.Operator)
								if err != nil {
									keep = false
									break
								}
							} else {
								// Regular value comparison
								resourceValue, filterValue, err := convertToComparableTypes(value, filter.Value)
								if err != nil {
									keep = false
									break
								}

								keep = compareValues(resourceValue, filterValue, filter.Operator)
							}

							if filter.IsNegated {
								keep = !keep
							}
						}

						if !keep {
							break
						}
					}
				} else if extraFilter.Type == "SubMatch" {
					subMatches = append(subMatches, extraFilter.SubMatch)
				}
			}

			if keep {
				filtered = append(filtered, resource)
			}
		}

		for _, subMatch := range subMatches {
			if subMatch.ReferenceNodeName != n.ResourceProperties.Name {
				continue
			}

			// for each submatch, run checkSubMatch
			subMatchResults, err := q.checkSubMatch(subMatch, n.ResourceProperties.Name)
			if err != nil {
				return fmt.Errorf("error checking submatch: %v", err)
			}

			// find the delta between subMatchResults and filtered
			// if subMatch.IsNegated, discard the delta
			// if not negated, keep the delta
			var newFiltered []map[string]interface{}
			// Get matched resources for this node if they exist
			matchedResources, hasMatches := subMatchResults["_ref_"+n.ResourceProperties.Name]

			for _, resource := range filtered {
				isMatch := hasMatches && isResourceInList(resource, matchedResources)
				// For negated patterns, keep resources that DON'T match
				// For non-negated patterns, keep resources that DO match
				if isMatch != subMatch.IsNegated {
					newFiltered = append(newFiltered, resource)
				}
			}
			filtered = newFiltered
		}

		// Lock for writing to both maps
		resultMapMutex.Lock()
		resultCache[cacheKey] = filtered
		resultMap[n.ResourceProperties.Name] = filtered
		resultMapMutex.Unlock()
	} else {
		// If we found it in cache, just copy to resultMap
		resultMapMutex.Lock()
		resultMap[n.ResourceProperties.Name] = cachedResult
		resultMapMutex.Unlock()
	}

	return nil
}

func compareValues(resourceValue, filterValue interface{}, operator string) bool {
	switch operator {
	case "EQUALS", "=", "==":
		return resourceValue == filterValue
	case "NOT_EQUALS", "!=":
		return resourceValue != filterValue
	case "GREATER_THAN", ">":
		if rv, ok := resourceValue.(float64); ok {
			if fv, ok := filterValue.(float64); ok {
				return rv > fv
			}
		}
	case "LESS_THAN", "<":
		if rv, ok := resourceValue.(float64); ok {
			if fv, ok := filterValue.(float64); ok {
				return rv < fv
			}
		}
	case "GREATER_THAN_EQUALS", ">=":
		if rv, ok := resourceValue.(float64); ok {
			if fv, ok := filterValue.(float64); ok {
				return rv >= fv
			}
		}
	case "LESS_THAN_EQUALS", "<=":
		if rv, ok := resourceValue.(float64); ok {
			if fv, ok := filterValue.(float64); ok {
				return rv <= fv
			}
		}
	case "CONTAINS":
		strA := fmt.Sprintf("%v", resourceValue)
		strB := fmt.Sprintf("%v", filterValue)
		return strings.Contains(strA, strB)
	case "REGEX_COMPARE":
		if filterValueStr, ok := filterValue.(string); ok {
			if resultValueStr, ok := resourceValue.(string); ok {
				if regex, err := regexp.Compile(filterValueStr); err == nil {
					return regex.MatchString(resultValueStr)
				}
			}
		}
	}
	return false
}

func GetContextQueryExecutor(context string) (*QueryExecutor, error) {
	executorsLock.RLock()
	if executor, exists := contextExecutors[context]; exists {
		executorsLock.RUnlock()
		return executor, nil
	}
	executorsLock.RUnlock()

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

	// Create a new provider for this context
	contextProvider, err := executorInstance.provider.CreateProviderForContext(context)
	if err != nil {
		return nil, fmt.Errorf("error creating provider for context %s: %v", context, err)
	}

	// Create new executor with the context-specific provider
	executor, err := NewQueryExecutor(contextProvider)
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

	// Let the provider handle caching internally
	// We'll just initialize an empty cache
	return nil
}

func InitResourceSpecs(p provider.Provider) error {
	if ResourceSpecs == nil {
		ResourceSpecs = make(map[string][]string)
	}

	logDebug("Getting OpenAPI resource specs...")
	specs, err := p.GetOpenAPIResourceSpecs()
	if err != nil {
		return fmt.Errorf("error getting resource specs: %w", err)
	}

	logDebug("Got specs for", len(specs), "resources")
	ResourceSpecs = specs

	return nil
}

func extractKindFromSchemaName(schemaName string) string {
	parts := strings.Split(schemaName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func logDebug(v ...interface{}) {
	if LogLevel == "debug" {
		fmt.Println(append([]interface{}{"[DEBUG] "}, v...)...)
	}
}

// Helper function to prefix variables in the AST
func prefixVariables(ast *Expression, context string) *Expression {
	modified := &Expression{
		Clauses:  make([]Clause, len(ast.Clauses)),
		Contexts: ast.Contexts,
	}

	for i, clause := range ast.Clauses {
		switch c := clause.(type) {
		case *MatchClause:
			modified.Clauses[i] = prefixMatchClause(c, context)
		case *ReturnClause:
			modified.Clauses[i] = prefixReturnClause(c, context)
		case *SetClause:
			modified.Clauses[i] = prefixSetClause(c, context)
		case *DeleteClause:
			modified.Clauses[i] = prefixDeleteClause(c, context)
		case *CreateClause:
			modified.Clauses[i] = prefixCreateClause(c, context)
		}
	}

	return modified
}

// Helper functions to prefix variables in each clause type
func prefixMatchClause(c *MatchClause, context string) *MatchClause {
	modified := &MatchClause{
		Nodes:         make([]*NodePattern, len(c.Nodes)),
		Relationships: make([]*Relationship, len(c.Relationships)),
		ExtraFilters:  make([]*Filter, len(c.ExtraFilters)),
	}

	// Prefix node names
	for i, node := range c.Nodes {
		modified.Nodes[i] = &NodePattern{
			ResourceProperties: &ResourceProperties{
				Name:       context + "_" + node.ResourceProperties.Name,
				Kind:       node.ResourceProperties.Kind,
				Properties: node.ResourceProperties.Properties,
				JsonData:   node.ResourceProperties.JsonData,
			},
		}
	}

	// Prefix relationships
	for i, rel := range c.Relationships {
		modified.Relationships[i] = &Relationship{
			ResourceProperties: rel.ResourceProperties,
			Direction:          rel.Direction,
			LeftNode: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name:       context + "_" + rel.LeftNode.ResourceProperties.Name,
					Kind:       rel.LeftNode.ResourceProperties.Kind,
					Properties: rel.LeftNode.ResourceProperties.Properties,
					JsonData:   rel.LeftNode.ResourceProperties.JsonData,
				},
			},
			RightNode: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name:       context + "_" + rel.RightNode.ResourceProperties.Name,
					Kind:       rel.RightNode.ResourceProperties.Kind,
					Properties: rel.RightNode.ResourceProperties.Properties,
					JsonData:   rel.RightNode.ResourceProperties.JsonData,
				},
			},
		}
	}

	// Prefix filter variables
	for i, extraFilter := range c.ExtraFilters {
		if extraFilter.Type == "KeyValuePair" {
			filter := extraFilter.KeyValuePair
			parts := strings.Split(filter.Key, ".")
			if len(parts) > 0 {
				parts[0] = context + "_" + parts[0]
			}
			modified.ExtraFilters[i] = &Filter{
				Type: "KeyValuePair",
				KeyValuePair: &KeyValuePair{
					Key:      strings.Join(parts, "."),
					Value:    filter.Value,
					Operator: filter.Operator,
				},
			}
		}
	}

	return modified
}

// Add similar prefix functions for other clause types...

func prefixReturnClause(c *ReturnClause, context string) *ReturnClause {
	modified := &ReturnClause{
		Items: make([]*ReturnItem, len(c.Items)),
	}

	for i, item := range c.Items {
		// Split the JsonPath to prefix the variable name
		parts := strings.Split(item.JsonPath, ".")
		if len(parts) > 0 {
			parts[0] = context + "_" + parts[0]
		}

		modified.Items[i] = &ReturnItem{
			JsonPath:  strings.Join(parts, "."),
			Alias:     item.Alias,
			Aggregate: item.Aggregate,
		}
	}

	return modified
}

func prefixSetClause(c *SetClause, context string) *SetClause {
	modified := &SetClause{
		KeyValuePairs: make([]*KeyValuePair, len(c.KeyValuePairs)),
	}

	for i, kvp := range c.KeyValuePairs {
		// Split the key to prefix the variable name
		parts := strings.Split(kvp.Key, ".")
		if len(parts) > 0 {
			parts[0] = context + "_" + parts[0]
		}

		modified.KeyValuePairs[i] = &KeyValuePair{
			Key:      strings.Join(parts, "."),
			Value:    kvp.Value,
			Operator: kvp.Operator,
		}
	}

	return modified
}

func prefixDeleteClause(c *DeleteClause, context string) *DeleteClause {
	modified := &DeleteClause{
		NodeIds: make([]string, len(c.NodeIds)),
	}

	for i, nodeId := range c.NodeIds {
		modified.NodeIds[i] = context + "_" + nodeId
	}

	return modified
}

func prefixCreateClause(c *CreateClause, context string) *CreateClause {
	modified := &CreateClause{
		Nodes:         make([]*NodePattern, len(c.Nodes)),
		Relationships: make([]*Relationship, len(c.Relationships)),
	}

	// Prefix node names
	for i, node := range c.Nodes {
		modified.Nodes[i] = &NodePattern{
			ResourceProperties: &ResourceProperties{
				Name:       context + "_" + node.ResourceProperties.Name,
				Kind:       node.ResourceProperties.Kind,
				Properties: node.ResourceProperties.Properties,
				JsonData:   node.ResourceProperties.JsonData,
			},
		}
	}

	// Prefix relationship node references
	for i, rel := range c.Relationships {
		modified.Relationships[i] = &Relationship{
			ResourceProperties: rel.ResourceProperties,
			Direction:          rel.Direction,
			LeftNode: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name:       context + "_" + rel.LeftNode.ResourceProperties.Name,
					Kind:       rel.LeftNode.ResourceProperties.Kind,
					Properties: rel.LeftNode.ResourceProperties.Properties,
					JsonData:   rel.LeftNode.ResourceProperties.JsonData,
				},
			},
			RightNode: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name:       context + "_" + rel.RightNode.ResourceProperties.Name,
					Kind:       rel.RightNode.ResourceProperties.Kind,
					Properties: rel.RightNode.ResourceProperties.Properties,
					JsonData:   rel.RightNode.ResourceProperties.JsonData,
				},
			},
		}
	}

	return modified
}

func evaluateWildcardPath(resource interface{}, path string, filterValue interface{}, operator string) bool {
	// Get the base path (everything before [*])
	basePath := path[:strings.Index(path, "[*]")]
	if !strings.HasPrefix(basePath, "$.") {
		basePath = "$." + basePath
	}

	// Get the array using the base path
	array, err := jsonpath.JsonPathLookup(resource, basePath)
	if err != nil {
		return false
	}

	// Convert to array of interfaces
	items, ok := array.([]interface{})
	if !ok {
		return false
	}

	// Get the remaining path after [*]
	remainingPath := path[strings.Index(path, "[*]")+3:]
	if remainingPath != "" && !strings.HasPrefix(remainingPath, ".") {
		remainingPath = "." + remainingPath
	}

	// Check each item in the array
	for _, item := range items {
		// For primitive array items
		if remainingPath == "" {
			itemValue, filterValue, err := convertToComparableTypes(item, filterValue)
			if err != nil {
				continue
			}
			if compareValues(itemValue, filterValue, operator) {
				return true
			}
			continue
		}

		// For object array items
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Create a new path for this item
		itemPath := "$" + remainingPath

		value, err := jsonpath.JsonPathLookup(itemMap, itemPath)
		if err != nil {
			continue
		}

		resourceValue, filterValue, err := convertToComparableTypes(value, filterValue)
		if err != nil {
			continue
		}

		if compareValues(resourceValue, filterValue, operator) {
			return true
		}
	}

	return false
}

// Update the SET clause handling to support wildcards
func (q *QueryExecutor) handleSetClause(c *SetClause) error {
	fmt.Printf("\n==== handleSetClause ====\n")
	fmt.Printf("Processing %d key-value pairs\n", len(c.KeyValuePairs))

	for i, kvp := range c.KeyValuePairs {
		fmt.Printf("\nProcessing key-value pair %d: %s = %v\n", i, kvp.Key, kvp.Value)

		// Extract the resource name (first part before any dot)
		parts := strings.SplitN(kvp.Key, ".", 2)
		resultMapKey := parts[0]
		fmt.Printf("Resource name: %s\n", resultMapKey)

		resources, ok := resultMap[resultMapKey].([]map[string]interface{})
		if !ok {
			fmt.Printf("Error: resultMap[%s] is not a slice of maps\n", resultMapKey)
			return fmt.Errorf("could not find resources for node %s in MATCH clause", resultMapKey)
		}
		fmt.Printf("Found %d resources for node %s\n", len(resources), resultMapKey)

		// Find the matching node from the stored match nodes
		var nodeKind string
		for _, node := range q.matchNodes {
			if node.ResourceProperties.Name == resultMapKey {
				nodeKind = node.ResourceProperties.Kind
				fmt.Printf("Found kind %s for node %s\n", nodeKind, resultMapKey)
				break
			}
		}
		if nodeKind == "" {
			fmt.Printf("Error: could not find kind for node %s\n", resultMapKey)
			return fmt.Errorf("could not find kind for node %s in MATCH clause", resultMapKey)
		}

		for j, resource := range resources {
			fmt.Printf("\nProcessing resource %d of %d\n", j+1, len(resources))

			if strings.Contains(kvp.Key, "[*]") {
				fmt.Printf("Detected wildcard path: %s\n", kvp.Key)
				// Handle wildcard updates
				err := applyWildcardUpdate(resource, kvp.Key, kvp.Value)
				if err != nil {
					fmt.Printf("Error applying wildcard update: %v\n", err)
					return err
				}
				fmt.Printf("Successfully applied wildcard update\n")
			} else {
				fmt.Printf("Processing regular path update\n")
				// Regular path update
				// First remove the resource name prefix (e.g., "d.")
				remainingPath := ""
				if len(parts) > 1 {
					remainingPath = parts[1]
				}
				fmt.Printf("Remaining path after removing resource prefix: %s\n", remainingPath)

				// Split the path handling escaped dots
				pathParts := splitEscapedPath(remainingPath)
				fmt.Printf("Path parts after splitting escaped dots: %v\n", pathParts)

				patches := createCompatiblePatch(pathParts, kvp.Value)
				patchJSON, err := json.Marshal(patches)
				if err != nil {
					fmt.Printf("Error marshalling patches: %v\n", err)
					return fmt.Errorf("error marshalling patches: %s", err)
				}
				fmt.Printf("Created patch JSON: %s\n", string(patchJSON))

				metadata, ok := resource["metadata"].(map[string]interface{})
				if !ok {
					fmt.Printf("Error: resource metadata is not a map\n")
					return fmt.Errorf("resource metadata is not a map")
				}

				name, ok := metadata["name"].(string)
				if !ok {
					fmt.Printf("Error: resource name is not a string\n")
					return fmt.Errorf("resource name is not a string")
				}

				namespace := getNamespaceName(metadata)
				fmt.Printf("Resource: %s/%s in namespace %s\n", nodeKind, name, namespace)

				fmt.Printf("Applying patch to resource %s/%s in namespace %s\n", nodeKind, name, namespace)
				logDebug("Patch JSON: %s", string(patchJSON))
				logDebug("Current resource state: %+v", resource)

				err = q.provider.PatchK8sResource(nodeKind, name, namespace, patchJSON)
				if err != nil {
					fmt.Printf("Error patching resource: %v\n", err)
					return fmt.Errorf("error patching resource: %s", err)
				}
				fmt.Printf("Successfully applied patch\n")

				// Verify the patch was applied
				fmt.Printf("Verifying patch was applied\n")
				updatedResource, err := q.provider.GetK8sResources(nodeKind, fmt.Sprintf("metadata.name=%s", name), "", namespace)
				if err != nil {
					fmt.Printf("Error verifying patch: %v\n", err)
					return fmt.Errorf("error verifying patch: %s", err)
				}
				fmt.Printf("Successfully verified patch\n")
				logDebug("Updated resource: %+v", updatedResource)
			}
		}
	}
	fmt.Printf("\n==== handleSetClause completed successfully ====\n")
	return nil
}

// splitEscapedPath splits a path by dots, but preserves escaped dots
func splitEscapedPath(path string) []string {
	var result []string
	var current strings.Builder
	escaped := false

	for i := 0; i < len(path); i++ {
		if escaped {
			// If we're in escaped mode and see a dot, add it to current
			if path[i] == '.' {
				current.WriteByte('.')
			} else {
				// Otherwise, write both the backslash and the current character
				current.WriteByte('\\')
				current.WriteByte(path[i])
			}
			escaped = false
		} else if path[i] == '\\' {
			// Enter escaped mode
			escaped = true
		} else if path[i] == '.' && !escaped {
			// Unescaped dot, split here
			result = append(result, current.String())
			current.Reset()
		} else {
			// Regular character
			current.WriteByte(path[i])
		}
	}

	// Add the last part
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

func (q *QueryExecutor) checkSubMatch(subMatch *SubMatch, referenceNodeName string) (map[string][]map[string]interface{}, error) {
	// Create temporary results and filtered results maps
	tempResults := QueryResult{
		Data: make(map[string]interface{}),
	}
	filteredResults := make(map[string][]map[string]interface{})
	resultMap := make(map[string][]map[string]interface{})

	for _, node := range subMatch.Nodes {
		if node.ResourceProperties.Name == referenceNodeName {
			node.ResourceProperties.Name = "_ref_" + node.ResourceProperties.Name
			break
		}
	}

	// also rename this node in the relationships
	for _, rel := range subMatch.Relationships {
		if rel.LeftNode.ResourceProperties.Name == referenceNodeName {
			rel.LeftNode.ResourceProperties.Name = "_ref_" + rel.LeftNode.ResourceProperties.Name
		}
		if rel.RightNode.ResourceProperties.Name == referenceNodeName {
			rel.RightNode.ResourceProperties.Name = "_ref_" + rel.RightNode.ResourceProperties.Name
		}
	}

	// Create a match clause for relationship processing
	matchClause := &MatchClause{
		Nodes:         subMatch.Nodes,
		Relationships: subMatch.Relationships,
	}

	// Get resources for the reference node first
	for _, node := range matchClause.Nodes {
		if node.ResourceProperties.Name == "_ref_"+subMatch.ReferenceNodeName {
			err := getNodeResources(node, q, nil)
			if err != nil {
				return nil, err
			}
			// Initialize filteredResults and resultMap with the reference node's resources
			filteredResults[node.ResourceProperties.Name] = getResourcesFromMap(filteredResults, node.ResourceProperties.Name)
			resultMap[node.ResourceProperties.Name] = getResourcesFromMap(filteredResults, node.ResourceProperties.Name)
			break
		}
	}

	// Process each relationship in the pattern, with multiple passes if needed
	for i := 0; i < len(matchClause.Relationships)*2; i++ {
		filteringOccurred := false

		for _, rel := range matchClause.Relationships {
			// Get resources for nodes that aren't the reference node
			if rel.LeftNode.ResourceProperties.Name != "_ref_"+subMatch.ReferenceNodeName {
				err := getNodeResources(rel.LeftNode, q, nil)
				if err != nil {
					return nil, err
				}
			}
			if rel.RightNode.ResourceProperties.Name != "_ref_"+subMatch.ReferenceNodeName {
				err := getNodeResources(rel.RightNode, q, nil)
				if err != nil {
					return nil, err
				}
			}

			// Process the relationship and update filteredResults
			filtered, err := q.processRelationship(rel, matchClause, &tempResults, filteredResults)
			if err != nil {
				return nil, err
			}
			filteringOccurred = filteringOccurred || filtered

			// Check if either side of the relationship has no results
			leftResults := filteredResults[rel.LeftNode.ResourceProperties.Name]
			rightResults := filteredResults[rel.RightNode.ResourceProperties.Name]
			if len(leftResults) == 0 || len(rightResults) == 0 {
				// If any part of the chain has no results, the entire pattern doesn't match
				return make(map[string][]map[string]interface{}), nil
			}
		}

		if !filteringOccurred {
			break
		}

		// Update resultMap with filtered results for the next pass
		for k, v := range filteredResults {
			resultMap[k] = v
		}
	}

	return filteredResults, nil
}

func isResourceInList(resource map[string]interface{}, matchedResources []map[string]interface{}) bool {
	resourceJSON, _ := json.Marshal(resource)
	for _, matchedResource := range matchedResources {
		matchedJSON, _ := json.Marshal(matchedResource)
		if string(resourceJSON) == string(matchedJSON) {
			return true
		}
	}
	return false
}

// Add this new function to handle wildcard updates
func applyWildcardUpdate(resource interface{}, path string, value interface{}) error {
	parts := strings.Split(path, "[*]")
	return applyWildcardUpdateRecursive(resource, parts, 0, value)
}

func applyWildcardUpdateRecursive(data interface{}, parts []string, depth int, value interface{}) error {
	if depth == len(parts)-1 {
		// Last part - set the value
		return setValueAtPath(data, parts[depth], value)
	}

	// Get the array at current level
	currentPath := parts[depth]
	if !strings.HasSuffix(currentPath, ".") {
		currentPath += "."
	}
	array, err := jsonpath.JsonPathLookup(data, currentPath)
	if err != nil {
		return err
	}

	// Update all elements in the array
	switch arr := array.(type) {
	case []interface{}:
		for _, item := range arr {
			if err := applyWildcardUpdateRecursive(item, parts, depth+1, value); err != nil {
				return err
			}
		}
	case []map[string]interface{}:
		for _, item := range arr {
			if err := applyWildcardUpdateRecursive(item, parts, depth+1, value); err != nil {
				return err
			}
		}
	}

	return nil
}

// Helper function to check if a node name corresponds to a kindless node
func isKindless(nodeName string, kindlessNodes []*NodePattern) bool {
	for _, node := range kindlessNodes {
		if node.ResourceProperties.Name == nodeName {
			return true
		}
	}
	return false
}
