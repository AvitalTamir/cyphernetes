package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/avitaltamir/cyphernetes/pkg/core"
)

// MockQueryExecutor implements the Execute method of QueryExecutor
type MockQueryExecutor struct {
	ExecuteFunc func(expr *core.Expression) (core.QueryResult, error)
}

func (m *MockQueryExecutor) Execute(expr *core.Expression) (core.QueryResult, error) {
	return m.ExecuteFunc(expr)
}

func TestRunQuery(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		parseQueryErr  error
		newExecutorErr error
		executeErr     error
		expectedOutput string
	}{
		{
			name: "Successful query",
			args: []string{"MATCH (n:Pod)"},
			expectedOutput: `{
  "test": "data"
}`,
		},
		{
			name:           "Parse query error",
			args:           []string{"INVALID QUERY"},
			parseQueryErr:  fmt.Errorf("parse error"),
			expectedOutput: "Error parsing query:  parse error",
		},
		{
			name:           "New executor error",
			args:           []string{"MATCH (n:Pod)"},
			newExecutorErr: fmt.Errorf("executor error"),
			expectedOutput: "Error creating query executor:  executor error",
		},
		{
			name:           "Execute error",
			args:           []string{"MATCH (n:Pod)"},
			executeErr:     fmt.Errorf("execution error"),
			expectedOutput: "Error executing query:  execution error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			originalParseQuery := parseQuery
			originalNewQueryExecutor := newQueryExecutor
			originalExecuteMethod := executeMethod

			parseQuery = func(query string) (*core.Expression, error) {
				if tt.parseQueryErr != nil {
					return nil, tt.parseQueryErr
				}
				return &core.Expression{}, nil
			}

			mockExecutor := &MockQueryExecutor{
				ExecuteFunc: func(expr *core.Expression) (core.QueryResult, error) {
					if tt.executeErr != nil {
						return core.QueryResult{}, tt.executeErr
					}
					return core.QueryResult{
						Data: map[string]interface{}{
							"test": "data",
						},
					}, nil
				},
			}

			newQueryExecutor = func() (*core.QueryExecutor, error) {
				if tt.newExecutorErr != nil {
					return nil, tt.newExecutorErr
				}
				return &core.QueryExecutor{}, nil
			}

			// Replace the Execute method
			executeMethod = func(qe *core.QueryExecutor, expr *core.Expression) (core.QueryResult, error) {
				return mockExecutor.Execute(expr)
			}

			// Restore original functions after test
			defer func() {
				parseQuery = originalParseQuery
				newQueryExecutor = originalNewQueryExecutor
				executeMethod = originalExecuteMethod
			}()

			// Execute the command
			buf := new(bytes.Buffer)

			runQuery(tt.args, buf)

			// Check the output
			got := strings.TrimSpace(buf.String())
			want := strings.TrimSpace(tt.expectedOutput)
			if got != want {
				t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
