// parser_test.go
package cmd

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// TestParseQueryWithReturn tests the parsing of a query with a MATCH and RETURN clause.
func TestParseQueryWithReturn(t *testing.T) {
	// Define the query to parse.
	query := `MATCH (d:deploy { service: "foo", app: "bar"})--(s:Service {service: "foo", app: "bar"}) RETURN s.spec.ports, d.metadata.name`

	// Define the expected AST structure after parsing.
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				NodePattern: &NodePattern{
					Name: "d",
					Kind: "deploy",
					Properties: &Properties{
						PropertyList: []*Property{
							{
								Key:   "service",
								Value: "foo",
							},
							{
								Key:   "app",
								Value: "bar",
							},
						},
					},
				},
				ConnectedNodePattern: &NodePattern{
					Name: "s",
					Kind: "Service",
					Properties: &Properties{
						PropertyList: []*Property{
							{
								Key:   "service",
								Value: "foo",
							},
							{
								Key:   "app",
								Value: "bar",
							},
						},
					},
				},
			},
			&ReturnClause{
				JsonPaths: []string{"s.spec.ports", "d.metadata.name"},
			},
		},
	}

	// Call the parser.
	expr, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}

	// Check if the resulting AST matches the expected structure.
	if !reflect.DeepEqual(expr, expected) {
		exprJson, _ := json.Marshal(expr)
		expectedJson, _ := json.Marshal(expected)
		fmt.Printf("expr: %+v\n", string(exprJson))
		fmt.Printf("expected: %+v\n", string(expectedJson))
		t.Errorf("ParseQuery() = %v, want %v", expr, expected)
	}
}

// TODO: Test invalid input to ensure the parser properly handles errors.
// func TestParseQueryWithReturnInvalid(t *testing.T) {
// 	invalidQueries := []string{
// 		"MATCH (k:Kind) RETURN",          // Missing jsonPath
// 		"MATCH (k:Kind) RETURN k.",       // Incomplete jsonPath
// 		"MATCH (k:Kind) RETURN k.(name)", // Invalid jsonPath
// 		// ... other invalid queries
// 	}

// 	for _, query := range invalidQueries {
// 		_, err := ParseQuery(query)
// 		if err == nil {
// 			t.Errorf("ParseQuery() with query %q; want error, got nil", query)
// 		}
// 	}
// }
