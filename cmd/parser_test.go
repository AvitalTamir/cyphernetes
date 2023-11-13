// parser_test.go
package cmd

import (
	"fmt"
	"reflect"
	"testing"
)

// TestParseQueryWithReturn tests the parsing of a query with a MATCH and RETURN clause.
func TestParseQueryWithReturn(t *testing.T) {
	fmt.Println("-------------------- TestParseQueryWithReturn --------------------")
	// Define the query to parse.
	query := "MATCH (k:Kind) RETURN k.name"

	// Define the expected AST structure after parsing.
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				NodePattern: &NodePattern{
					Name: "k",
					Kind: "Kind",
				},
			},
			&ReturnClause{
				JsonPath: "k.name",
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
