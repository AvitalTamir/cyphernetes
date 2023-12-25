// parser_test.go
package parser

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
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
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
					},
					{
						ResourceProperties: &ResourceProperties{
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
				Relationships: []*Relationship{},
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
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "n",
							Kind: "Node",
						},
					},
				},
				Relationships: []*Relationship{},
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
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "n",
							Kind: "Node",
						},
					},
					{
						ResourceProperties: &ResourceProperties{
							Name: "m",
							Kind: "Module",
						},
					},
				},
				Relationships: []*Relationship{},
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
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "n",
							Kind: "Node",
						},
					},
					{
						ResourceProperties: &ResourceProperties{
							Name: "m",
							Kind: "Module",
						},
					},
				},
				Relationships: []*Relationship{
					{
						Direction: Right,
						LeftNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "n",
								Kind: "Node",
							},
						},
						RightNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "m",
								Kind: "Module",
							},
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
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "a",
							Kind: "App",
						},
					},
					{
						ResourceProperties: &ResourceProperties{
							Name: "b",
							Kind: "Backend",
						},
					},
					{
						ResourceProperties: &ResourceProperties{
							Name: "c",
							Kind: "Cache",
						},
					},
				},
				Relationships: []*Relationship{
					{
						Direction: Right,
						LeftNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "a",
								Kind: "App",
							},
						},
						RightNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "b",
								Kind: "Backend",
							},
						},
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
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "a",
							Kind: "App",
						},
					},
					{
						ResourceProperties: &ResourceProperties{
							Name: "b",
							Kind: "Backend",
						},
					},
					{
						ResourceProperties: &ResourceProperties{
							Name: "d",
							Kind: "Database",
						},
					},
				},
				Relationships: []*Relationship{
					{
						Direction: Right,
						LeftNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "a",
								Kind: "App",
							},
						},
						RightNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "b",
								Kind: "Backend",
							},
						},
					},
					{
						Direction: Right,
						LeftNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "b",
								Kind: "Backend",
							},
						},
						RightNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "d",
								Kind: "Database",
							},
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
	query := `MATCH (a:App)->(b:Backend)-[r:Relationship {test: "foo"}]->(d:Database), (c:Cache) RETURN a,b,c,d`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "a",
							Kind: "App",
						},
					},
					{
						ResourceProperties: &ResourceProperties{
							Name: "b",
							Kind: "Backend",
						},
					},
					{
						ResourceProperties: &ResourceProperties{
							Name: "d",
							Kind: "Database",
						},
					},
					{
						ResourceProperties: &ResourceProperties{
							Name: "c",
							Kind: "Cache",
						},
					},
				},
				Relationships: []*Relationship{
					{
						Direction: Right,
						LeftNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "a",
								Kind: "App",
							},
						},
						RightNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "b",
								Kind: "Backend",
							},
						},
					},
					{
						Direction: Right,
						LeftNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "b",
								Kind: "Backend",
							},
						},
						RightNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "d",
								Kind: "Database",
							},
						},
						ResourceProperties: &ResourceProperties{
							Name: "r",
							Kind: "Relationship",
							Properties: &Properties{
								PropertyList: []*Property{
									{
										Key:   "test",
										Value: "foo",
									},
								},
							},
						},
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

func TestMatchSetExpression(t *testing.T) {
	query := `MATCH (n:Node) SET n.name = "test"`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "n",
							Kind: "Node",
						},
					},
				},
				Relationships: []*Relationship{},
			},
			&SetClause{
				KeyValuePairs: []*KeyValuePair{
					{
						Key:   "n.name",
						Value: "test",
					},
				},
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

func TestMatchDeleteExpression(t *testing.T) {
	query := `MATCH (n:Node) DELETE n`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "n",
							Kind: "Node",
						},
					},
				},
				Relationships: []*Relationship{},
			},
			&DeleteClause{
				NodeIds: []string{"n"},
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
		fmt.Printf("expr: %+v\n", expr)
		fmt.Printf("expected: %+v\n", expected)
		t.Errorf("ParseQuery() = %v, want %v", expr, expected)
	}
}

func TestMatchCreateExpression(t *testing.T) {
	query := `MATCH (n:Node) CREATE (n)->(n2:Node)`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "n",
							Kind: "Node",
						},
					},
				},
				Relationships: []*Relationship{},
			},
			&CreateClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "n",
							Kind: "",
						},
					},
					{
						ResourceProperties: &ResourceProperties{
							Name: "n2",
							Kind: "Node",
						},
					},
				},
				Relationships: []*Relationship{
					{
						Direction: Right,
						LeftNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "n",
								Kind: "",
							},
						},
						RightNode: &NodePattern{
							ResourceProperties: &ResourceProperties{
								Name: "n2",
								Kind: "Node",
							},
						},
					},
				},
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
