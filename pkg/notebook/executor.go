package notebook

import (
	"encoding/json"
	"fmt"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

// QueryExecutor handles query execution for notebook cells
type QueryExecutor struct {
	executor *core.QueryExecutor
}

// QueryResult represents the result of a query execution
type QueryResult struct {
	Data  interface{} `json:"data"`
	Graph interface{} `json:"graph"`
	Error string      `json:"error,omitempty"`
}

// NewQueryExecutor creates a new query executor instance
func NewQueryExecutor(dryRun bool) (*QueryExecutor, error) {
	// Create API server provider config
	providerConfig := &apiserver.APIServerProviderConfig{
		DryRun: dryRun,
	}
	
	// Create provider
	provider, err := apiserver.NewAPIServerProviderWithOptions(providerConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating provider: %w", err)
	}

	// Get executor instance
	executor := core.GetQueryExecutorInstance(provider)
	if executor == nil {
		return nil, fmt.Errorf("failed to initialize query executor")
	}

	return &QueryExecutor{
		executor: executor,
	}, nil
}

// ExecuteQuery executes a Cyphernetes query and returns the results
func (qe *QueryExecutor) ExecuteQuery(query string, namespace string) (*QueryResult, error) {
	// Set namespace if provided
	if namespace != "" {
		core.Namespace = namespace
	}

	// Parse the query
	ast, err := core.ParseQuery(query)
	if err != nil {
		return &QueryResult{
			Error: fmt.Sprintf("Parse error: %v", err),
		}, nil
	}

	// Execute the query
	result, err := qe.executor.Execute(ast, core.Namespace)
	if err != nil {
		return &QueryResult{
			Error: fmt.Sprintf("Execution error: %v", err),
		}, nil
	}

	// Return the structured result
	return &QueryResult{
		Data:  result.Data,
		Graph: result.Graph,
	}, nil
}

// ExecuteQueryWithOptions executes a query with additional options
func (qe *QueryExecutor) ExecuteQueryWithOptions(query string, options QueryOptions) (*QueryResult, error) {
	// Set global options
	if options.Namespace != "" {
		core.Namespace = options.Namespace
	}
	if options.AllNamespaces {
		core.AllNamespaces = true
	}
	if options.NoColor {
		core.NoColor = true
	}

	// Execute the query
	result, err := qe.ExecuteQuery(query, options.Namespace)
	
	// Reset global options to avoid side effects
	if options.AllNamespaces {
		core.AllNamespaces = false
	}
	if options.NoColor {
		core.NoColor = false
	}

	return result, err
}

// QueryOptions provides additional configuration for query execution
type QueryOptions struct {
	Namespace     string `json:"namespace"`
	AllNamespaces bool   `json:"all_namespaces"`
	NoColor       bool   `json:"no_color"`
}

// ValidateQuery validates a query without executing it
func (qe *QueryExecutor) ValidateQuery(query string) error {
	_, err := core.ParseQuery(query)
	return err
}

// GetQueryResultAsJSON returns the query result as JSON bytes
func (qe *QueryExecutor) GetQueryResultAsJSON(result *QueryResult) ([]byte, error) {
	return json.Marshal(result)
}