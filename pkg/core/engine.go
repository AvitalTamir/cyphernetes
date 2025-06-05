package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

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

type QueryExecutor struct {
	provider       provider.Provider
	requestChannel chan *apiRequest
	semaphore      chan struct{}
	matchNodes     []*NodePattern
	currentAst     *Expression
}

var (
	executorInstance *QueryExecutor
	contextExecutors map[string]*QueryExecutor
	once             sync.Once
	GvrCache         map[string]schema.GroupVersionResource
	ResourceSpecs    map[string][]string
	executorsLock    sync.RWMutex
	resultMapMutex   sync.RWMutex
	Namespace        string
	LogLevel         string
	OutputFormat     string
	AllNamespaces    bool
	CleanOutput      bool
	NoColor          bool
	// For testing
	mockFindPotentialKinds func([]*Relationship) []string
)

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

func GetQueryExecutorInstance(p provider.Provider, relationshipRules ...RelationshipRule) *QueryExecutor {
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
		InitializeRelationships(ResourceSpecs, p, relationshipRules...)
	})
	return executorInstance
}

func (q *QueryExecutor) Provider() provider.Provider {
	return q.provider
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
