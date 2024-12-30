package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider"
)

// MockQueryExecutor implements the QueryExecutor interface
type MockQueryExecutor struct {
	ExecuteFunc func(expr *core.Expression, namespace string) (core.QueryResult, error)
}

func (m *MockQueryExecutor) Execute(expr *core.Expression, namespace string) (core.QueryResult, error) {
	return m.ExecuteFunc(expr, namespace)
}

func (m *MockQueryExecutor) Provider() provider.Provider {
	return &MockProvider{}
}

// Add a mock provider
type MockProvider struct {
	provider.Provider
}

func TestRunQuery(t *testing.T) {
	// Store original functions to restore later
	originalParseQuery := parseQuery
	originalNewQueryExecutor := newQueryExecutor

	tests := []struct {
		name            string
		args            []string
		setup           func()
		wantOut         string
		mockParseQuery  func(string) (*core.Expression, error)
		mockExecute     func(*core.Expression, string) (core.QueryResult, error)
		mockNewExecutor func(provider.Provider) (*core.QueryExecutor, error)
	}{
		{
			name: "New executor error",
			args: []string{"match (p:pods)"},
			mockNewExecutor: func(p provider.Provider) (*core.QueryExecutor, error) {
				return nil, fmt.Errorf("executor error")
			},
			wantOut: "Error creating query executor:  executor error\n",
		},
		{
			name: "Successful query",
			args: []string{"MATCH (n:Pod)"},
			mockParseQuery: func(query string) (*core.Expression, error) {
				return &core.Expression{}, nil
			},
			mockExecute: func(expr *core.Expression, namespace string) (core.QueryResult, error) {
				return core.QueryResult{
					Data: map[string]interface{}{
						"test": "data",
					},
				}, nil
			},
			wantOut: `{
  "test": "data"
}
`,
		},
		{
			name: "Successful query in YAML format",
			args: []string{"MATCH (n:Pod)"},
			mockParseQuery: func(query string) (*core.Expression, error) {
				return &core.Expression{}, nil
			},
			mockExecute: func(expr *core.Expression, namespace string) (core.QueryResult, error) {
				core.OutputFormat = "yaml"
				return core.QueryResult{
					Data: map[string]interface{}{
						"test": "data",
					},
				}, nil
			},
			wantOut: "test: data\n\n",
		},
		{
			name: "Parse query error",
			args: []string{"INVALID QUERY"},
			mockParseQuery: func(query string) (*core.Expression, error) {
				return nil, fmt.Errorf("parse error")
			},
			wantOut: "Error parsing query:  parse error\n",
		},
		{
			name: "Execute error",
			args: []string{"MATCH (n:Pod)"},
			mockParseQuery: func(query string) (*core.Expression, error) {
				return &core.Expression{}, nil
			},
			mockExecute: func(expr *core.Expression, namespace string) (core.QueryResult, error) {
				return core.QueryResult{}, fmt.Errorf("execution error")
			},
			wantOut: "Error executing query:  execution error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			// Set up mocks for this test
			if tt.mockParseQuery != nil {
				parseQuery = tt.mockParseQuery
			}
			if tt.mockNewExecutor != nil {
				newQueryExecutor = tt.mockNewExecutor
			}
			if tt.mockExecute != nil {
				newQueryExecutor = func(p provider.Provider) (*core.QueryExecutor, error) {
					return &core.QueryExecutor{}, nil
				}
				executeMethod = func(_ *core.QueryExecutor, expr *core.Expression, namespace string) (core.QueryResult, error) {
					return tt.mockExecute(expr, namespace)
				}
			}

			out := &bytes.Buffer{}
			runQuery(tt.args, out)

			if got := out.String(); got != tt.wantOut {
				t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", got, tt.wantOut)
			}

			// Reset mocks after test
			parseQuery = originalParseQuery
			newQueryExecutor = originalNewQueryExecutor
		})
	}
}
