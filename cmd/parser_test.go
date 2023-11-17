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
	query := `MATCH (d:deploy { service: "foo", app: "bar"}), (s:Service {service: "foo", app: "bar"}) RETURN s.spec.ports, d.metadata.name`

	// Define the expected AST structure after parsing.
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				NodePatternList: []*NodePattern{
					{
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
					{
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

func TestSingleNodePattern(t *testing.T) {
	query := `MATCH (n:Node) RETURN n`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				NodePatternList: []*NodePattern{
					{
						Name: "n",
						Kind: "Node",
					},
				},
			},
			&ReturnClause{
				JsonPaths: []string{"n"},
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

func TestMultipleNodePatternsCommaSeparated(t *testing.T) {
	query := `MATCH (n:Node), (m:Module) RETURN n,m`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				NodePatternList: []*NodePattern{
					{
						Name: "n",
						Kind: "Node",
					},
					{
						Name: "m",
						Kind: "Module",
					},
				},
			},
			&ReturnClause{
				JsonPaths: []string{"n", "m"},
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

func TestMultipleNodePatternsRelationship(t *testing.T) {
	query := `MATCH (n:Node)->(m:Module) RETURN n,m`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				NodePatternList: []*NodePattern{
					{
						Name: "n",
						Kind: "Node",
						ConnectedNodePatternRight: &NodePattern{
							Name: "m",
							Kind: "Module",
						},
					},
					{
						Name: "m",
						Kind: "Module",
						ConnectedNodePatternLeft: &NodePattern{
							Name: "n",
							Kind: "Node",
						},
					},
				},
			},
			&ReturnClause{
				JsonPaths: []string{"n", "m"},
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

func TestComplexNodePatternsAndRelationships(t *testing.T) {
	query := `MATCH (a:App)->(b:Backend), (c:Cache) RETURN a,b,c`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				NodePatternList: []*NodePattern{
					{
						Name: "a",
						Kind: "App",
						ConnectedNodePatternRight: &NodePattern{
							Name: "b",
							Kind: "Backend",
						},
					},
					{
						Name: "b",
						Kind: "Backend",
						ConnectedNodePatternLeft: &NodePattern{
							Name: "a",
							Kind: "App",
						},
					},
					{
						Name: "c",
						Kind: "Cache",
					},
				},
			},
			&ReturnClause{
				JsonPaths: []string{"a", "b", "c"},
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

func TestChainedRelationships(t *testing.T) {
	query := `MATCH (a:App)->(b:Backend)->(d:Database) RETURN a,b,d`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				NodePatternList: []*NodePattern{
					{
						Name: "a",
						Kind: "App",
						ConnectedNodePatternRight: &NodePattern{
							Name: "b",
							Kind: "Backend",
						},
					},
					{
						Name: "b",
						Kind: "Backend",
						ConnectedNodePatternLeft: &NodePattern{
							Name: "a",
							Kind: "App",
						},
						ConnectedNodePatternRight: &NodePattern{
							Name: "d",
							Kind: "Database",
						},
					},
					{
						Name: "d",
						Kind: "Database",
						ConnectedNodePatternLeft: &NodePattern{
							Name: "b",
							Kind: "Backend",
						},
					},
				},
			},
			&ReturnClause{
				JsonPaths: []string{"a", "b", "d"},
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

func TestChainedRelationshipsWithComma(t *testing.T) {
	query := `MATCH (a:App)->(b:Backend)->(d:Database), (c:Cache) RETURN a,b,c,d`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				NodePatternList: []*NodePattern{
					{
						Name: "a",
						Kind: "App",
						ConnectedNodePatternRight: &NodePattern{
							Name: "b",
							Kind: "Backend",
						},
					},
					{
						Name: "b",
						Kind: "Backend",
						ConnectedNodePatternLeft: &NodePattern{
							Name: "a",
							Kind: "App",
						},
						ConnectedNodePatternRight: &NodePattern{
							Name: "d",
							Kind: "Database",
						},
					},
					{
						Name: "d",
						Kind: "Database",
						ConnectedNodePatternLeft: &NodePattern{
							Name: "b",
							Kind: "Backend",
						},
					},
					{
						Name: "c",
						Kind: "Cache",
					},
				},
			},
			&ReturnClause{
				JsonPaths: []string{"a", "b", "c", "d"},
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
