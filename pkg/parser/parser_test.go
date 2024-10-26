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
	query := `MATCH (d:deploy { service: "foo", app: "bar"}), (s:Service {service: "foo", app: "bar", "test.io/test": "foo"}) RETURN s.spec.ports, d.metadata.name`

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
									{
										Key:   `"test.io/test"`,
										Value: "foo",
									},
								},
							},
						},
					},
				},
				Relationships: []*Relationship{},
				ExtraFilters:  nil,
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "s.spec.ports"},
					{JsonPath: "d.metadata.name"},
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

func TestParseQueryWithReturnAndAlias(t *testing.T) {
	query := `MATCH (d:deploy) RETURN d.metadata.name AS deploymentName, d.spec.replicas AS replicaCount`

	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "d",
							Kind: "deploy",
						},
					},
				},
				Relationships: []*Relationship{},
				ExtraFilters:  nil,
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "d.metadata.name", Alias: "deploymentName"},
					{JsonPath: "d.spec.replicas", Alias: "replicaCount"},
				},
			},
		},
	}

	expr, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}

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
				ExtraFilters:  nil,
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "n"},
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
				ExtraFilters:  nil,
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "n"},
					{JsonPath: "m"},
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
				ExtraFilters: nil,
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "n"},
					{JsonPath: "m"},
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
				ExtraFilters: nil,
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "a"},
					{JsonPath: "b"},
					{JsonPath: "c"},
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
				ExtraFilters: nil,
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "a"},
					{JsonPath: "b"},
					{JsonPath: "d"},
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
				ExtraFilters: nil,
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "a"},
					{JsonPath: "b"},
					{JsonPath: "c"},
					{JsonPath: "d"},
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
				ExtraFilters:  nil,
			},
			&SetClause{
				KeyValuePairs: []*KeyValuePair{
					{
						Key:      "n.name",
						Value:    "test",
						Operator: "EQUALS",
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
				ExtraFilters:  nil,
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
				ExtraFilters:  nil,
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

func TestMatchWhereReturn(t *testing.T) {
	query := `MATCH (k:Kind) WHERE k.name = "test" RETURN k.name`
	// Expected AST structure...
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "k",
							Kind: "Kind",
						},
					},
				},
				Relationships: []*Relationship{},
				ExtraFilters: []*KeyValuePair{
					{
						Key:      "k.name",
						Value:    "test",
						Operator: "EQUALS",
					},
				},
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "k.name"},
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

func TestMatchWhereReturnAs(t *testing.T) {
	query := `MATCH (k:Kind) WHERE k.name = "test" RETURN k.name AS kindName`
	// Expected AST structure
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "k",
							Kind: "Kind",
						},
					},
				},
				Relationships: []*Relationship{},
				ExtraFilters: []*KeyValuePair{
					{
						Key:      "k.name",
						Value:    "test",
						Operator: "EQUALS",
					},
				},
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "k.name", Alias: "kindName"},
				},
			},
		},
	}

	// Call the parser
	expr, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}

	// Check if the resulting AST matches the expected structure
	if !reflect.DeepEqual(expr, expected) {
		t.Errorf("ParseQuery() = %v, want %v", expr, expected)
	}
}

// Add these new test functions to the existing parser_test.go file

func TestParseQueryWithNotEquals(t *testing.T) {
	query := `MATCH (d:Deployment) WHERE d.spec.replicas != 3 RETURN d.metadata.name`
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "d",
							Kind: "Deployment",
						},
					},
				},
				Relationships: []*Relationship{},
				ExtraFilters: []*KeyValuePair{
					{
						Key:      "d.spec.replicas",
						Value:    3,
						Operator: "NOT_EQUALS",
					},
				},
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "d.metadata.name"},
				},
			},
		},
	}

	testParseQuery(t, query, expected)
}

func TestParseQueryWithGreaterThan(t *testing.T) {
	query := `MATCH (p:Pod) WHERE p.spec.containers[0].resources.limits.cpu > "500m" RETURN p.metadata.name`
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "p",
							Kind: "Pod",
						},
					},
				},
				Relationships: []*Relationship{},
				ExtraFilters: []*KeyValuePair{
					{
						Key:      "p.spec.containers[0].resources.limits.cpu",
						Value:    "500m",
						Operator: "GREATER_THAN",
					},
				},
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "p.metadata.name"},
				},
			},
		},
	}

	testParseQuery(t, query, expected)
}

func TestParseQueryWithLessThan(t *testing.T) {
	query := `MATCH (n:Node) WHERE n.status.allocatable.memory < "16Gi" RETURN n.metadata.name`
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
				ExtraFilters: []*KeyValuePair{
					{
						Key:      "n.status.allocatable.memory",
						Value:    "16Gi",
						Operator: "LESS_THAN",
					},
				},
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "n.metadata.name"},
				},
			},
		},
	}

	testParseQuery(t, query, expected)
}

func TestParseQueryWithGreaterThanOrEqual(t *testing.T) {
	query := `MATCH (d:Deployment) WHERE d.spec.replicas >= 5 RETURN d.metadata.name`
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "d",
							Kind: "Deployment",
						},
					},
				},
				Relationships: []*Relationship{},
				ExtraFilters: []*KeyValuePair{
					{
						Key:      "d.spec.replicas",
						Value:    5,
						Operator: "GREATER_THAN_EQUALS",
					},
				},
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "d.metadata.name"},
				},
			},
		},
	}

	testParseQuery(t, query, expected)
}

func TestParseQueryWithLessThanOrEqual(t *testing.T) {
	query := `MATCH (p:Pod) WHERE p.spec.containers[0].resources.requests.memory <= "512Mi" RETURN p.metadata.name`
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "p",
							Kind: "Pod",
						},
					},
				},
				Relationships: []*Relationship{},
				ExtraFilters: []*KeyValuePair{
					{
						Key:      "p.spec.containers[0].resources.requests.memory",
						Value:    "512Mi",
						Operator: "LESS_THAN_EQUALS",
					},
				},
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "p.metadata.name"},
				},
			},
		},
	}

	testParseQuery(t, query, expected)
}

func TestParseQueryWithMultipleComparisons(t *testing.T) {
	query := `MATCH (d:Deployment) WHERE d.spec.replicas > 2, d.metadata.name != "default", d.spec.template.spec.containers[0].resources.limits.cpu <= "1" RETURN d.metadata.name, d.spec.replicas`
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "d",
							Kind: "Deployment",
						},
					},
				},
				Relationships: []*Relationship{},
				ExtraFilters: []*KeyValuePair{
					{
						Key:      "d.spec.replicas",
						Value:    2,
						Operator: "GREATER_THAN",
					},
					{
						Key:      "d.metadata.name",
						Value:    "default",
						Operator: "NOT_EQUALS",
					},
					{
						Key:      "d.spec.template.spec.containers[0].resources.limits.cpu",
						Value:    "1",
						Operator: "LESS_THAN_EQUALS",
					},
				},
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "d.metadata.name"},
					{JsonPath: "d.spec.replicas"},
				},
			},
		},
	}

	testParseQuery(t, query, expected)
}

// Helper function to reduce boilerplate in test cases
func testParseQuery(t *testing.T, query string, expected *Expression) {
	expr, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}

	if !reflect.DeepEqual(expr, expected) {
		exprJson, _ := json.Marshal(expr)
		expectedJson, _ := json.Marshal(expected)
		fmt.Printf("expr: %+v\n", string(exprJson))
		fmt.Printf("expected: %+v\n", string(expectedJson))
		t.Errorf("ParseQuery() = %v, want %v", expr, expected)
	}
}

func TestMatchReturnSumAsCountAs(t *testing.T) {
	query := `MATCH (k:Kind) RETURN SUM{k.age} AS totalAge, COUNT{k.name} AS kindCount`
	// Expected AST structure
	expected := &Expression{
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					{
						ResourceProperties: &ResourceProperties{
							Name: "k",
							Kind: "Kind",
						},
					},
				},
				Relationships: []*Relationship{},
				ExtraFilters:  nil,
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{
						Aggregate: "SUM",
						JsonPath:  "k.age",
						Alias:     "totalAge",
					},
					{
						Aggregate: "COUNT",
						JsonPath:  "k.name",
						Alias:     "kindCount",
					},
				},
			},
		},
	}

	// Call the parser
	expr, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}

	// Check if the resulting AST matches the expected structure
	if !reflect.DeepEqual(expr, expected) {
		t.Errorf("ParseQuery() = %v, want %v", expr, expected)
	}
}

func TestParseQueryWithContainsString(t *testing.T) {
	query := `MATCH (n:Node) WHERE n.metadata.name CONTAINS "test" RETURN n.metadata.name`
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
				ExtraFilters: []*KeyValuePair{
					{
						Key:      "n.metadata.name",
						Value:    "test",
						Operator: "CONTAINS",
					},
				},
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "n.metadata.name"},
				},
			},
		},
	}

	testParseQuery(t, query, expected)
}

func TestParseQueryWithRegexCompare(t *testing.T) {
	query := `MATCH (n:Node) WHERE n.metadata.name =~ "^test.*" RETURN n.metadata.name`
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
				ExtraFilters: []*KeyValuePair{
					{
						Key:      "n.metadata.name",
						Value:    "^test.*",
						Operator: "REGEX_COMPARE",
					},
				},
			},
			&ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "n.metadata.name"},
				},
			},
		},
	}

	testParseQuery(t, query, expected)
}
