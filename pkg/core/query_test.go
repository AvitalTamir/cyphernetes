package core

import (
	"reflect"
	"strings"
	"testing"
)

// MockRelationshipResolver is used for testing
type MockRelationshipResolver struct {
	potentialKindsByNode map[string][]string
}

func (m *MockRelationshipResolver) FindPotentialKindsIntersection(relationships []*Relationship) []string {
	// If no mapping is provided, return empty slice
	if len(m.potentialKindsByNode) == 0 {
		return []string{}
	}

	// Find the first kindless node and get its potential kinds
	var result []string
	for _, rel := range relationships {
		if rel.LeftNode.ResourceProperties.Kind == "" {
			if kinds, ok := m.potentialKindsByNode[rel.LeftNode.ResourceProperties.Name]; ok {
				result = kinds
				break
			}
		}
		if rel.RightNode.ResourceProperties.Kind == "" {
			if kinds, ok := m.potentialKindsByNode[rel.RightNode.ResourceProperties.Name]; ok {
				result = kinds
				break
			}
		}
	}

	// For each additional kindless node, intersect its potential kinds with the result
	for _, rel := range relationships {
		if rel.LeftNode.ResourceProperties.Kind == "" {
			if kinds, ok := m.potentialKindsByNode[rel.LeftNode.ResourceProperties.Name]; ok {
				if result == nil {
					result = kinds
				} else {
					result = intersectKinds(result, kinds)
				}
			}
		}
		if rel.RightNode.ResourceProperties.Kind == "" {
			if kinds, ok := m.potentialKindsByNode[rel.RightNode.ResourceProperties.Name]; ok {
				if result == nil {
					result = kinds
				} else {
					result = intersectKinds(result, kinds)
				}
			}
		}
	}

	return result
}

// Helper function to find intersection of two string slices
func intersectKinds(a, b []string) []string {
	set := make(map[string]bool)
	for _, k := range a {
		set[k] = true
	}

	var result []string
	for _, k := range b {
		if set[k] {
			result = append(result, k)
		}
	}
	return result
}

func TestRewriteQueryForKindlessNodes(t *testing.T) {
	mockProvider := &MockProvider{}
	tests := []struct {
		name          string
		query         string
		mockKinds     map[string][]string
		expectedQuery string
		expectedError bool
		errorContains string
	}{
		{
			name:          "No kindless nodes",
			query:         "MATCH (d:Deployment)->(p:Pod) RETURN d, p",
			mockKinds:     map[string][]string{},
			expectedQuery: "",
			expectedError: false,
		},
		{
			name:          "Single kindless node with one potential kind",
			query:         "MATCH (d:Deployment)->(x) RETURN d, x",
			mockKinds:     map[string][]string{"x": {"Pod"}},
			expectedQuery: "MATCH (d__exp__0:Deployment)->(x__exp__0:Pod) RETURN d__exp__0, x__exp__0",
			expectedError: false,
		},
		{
			name:          "Single kindless node with multiple potential kinds",
			query:         "MATCH (d:Deployment)->(x) RETURN d, x",
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: "MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet) RETURN d__exp__0, x__exp__0, d__exp__1, x__exp__1",
			expectedError: false,
		},
		{
			name:          "Multiple kindless nodes with same potential kind",
			query:         "MATCH (d:Deployment)->(x), (s:Service)->(x) RETURN d, s, x",
			mockKinds:     map[string][]string{"x": {"Pod"}},
			expectedQuery: "MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (s__exp__0:Service)->(x__exp__0:Pod) RETURN d__exp__0, s__exp__0, x__exp__0",
			expectedError: false,
		},
		{
			name:          "Multiple kindless nodes with different potential kinds",
			query:         "MATCH (d:Deployment)->(x), (s:Service)->(y) RETURN d, s, x, y",
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}, "y": {"Pod", "Endpoints"}},
			expectedQuery: "MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (s__exp__0:Service)->(y__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet), (s__exp__1:Service)->(y__exp__1:Endpoints) RETURN d__exp__0, s__exp__0, x__exp__0, y__exp__0, d__exp__1, s__exp__1, x__exp__1, y__exp__1",
			expectedError: false,
		},
		{
			name:          "Multiple kindless nodes with intersecting potential kinds",
			query:         "MATCH (d:Deployment)->(x), (s:Service)->(x) RETURN d, s, x",
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: "MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (s__exp__0:Service)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet), (s__exp__1:Service)->(x__exp__1:ReplicaSet) RETURN d__exp__0, s__exp__0, x__exp__0, d__exp__1, s__exp__1, x__exp__1",
			expectedError: false,
		},
		{
			name:          "Kindless node with properties",
			query:         `MATCH (d:Deployment)->(x {name: "test"}) RETURN d, x`,
			mockKinds:     map[string][]string{"x": {"Pod"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod {name: "test"}) RETURN d__exp__0, x__exp__0`,
			expectedError: false,
		},
		{
			name:          "Match/Set/Return with multiple potential kinds",
			query:         `MATCH (d:Deployment)->(x) SET x.metadata.labels.foo = "bar" RETURN d, x`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet) SET x__exp__0.metadata.labels.foo = "test", x__exp__1.metadata.labels.foo = "test" RETURN d__exp__0, x__exp__0, d__exp__1, x__exp__1`,
			expectedError: false,
		},
		{
			name:          "Match/Where/Return with node properties and multiple potential kinds",
			query:         `MATCH (d:Deployment)->(x {name: "test"}) WHERE x.metadata.labels.foo = "bar" RETURN d, x`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod {name: "test"}), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet {name: "test"}) WHERE x__exp__0.metadata.labels.foo = "bar", x__exp__1.metadata.labels.foo = "bar" RETURN d__exp__0, x__exp__0, d__exp__1, x__exp__1`,
			expectedError: false,
		},
		{
			name:          "Match/Delete with node properties and multiple potential kinds",
			query:         `MATCH (d:Deployment)->(x {name: "test"}) DELETE x`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod {name: "test"}), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet {name: "test"}) DELETE x__exp__0, x__exp__1`,
			expectedError: false,
		},
		{
			name:          "Match/Return with aggregation",
			query:         `MATCH (d:Deployment)->(x) RETURN COUNT {d}, SUM {x}`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet) RETURN COUNT {d__exp__0}, SUM {x__exp__0}, COUNT {d__exp__1}, SUM {x__exp__1}`,
			expectedError: false,
		},
		{
			name:          "Multiple kindless nodes with different potential kinds returning a mixture of aggregation and non-aggregation with properties",
			query:         `MATCH (d:Deployment {name: "test"})->(x), (s:Service)->(y) RETURN d, COUNT {s}, x, SUM {y.spec.replicas}`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}, "y": {"Pod", "Endpoints"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (s__exp__0:Service)->(y__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet), (s__exp__1:Service)->(y__exp__1:Endpoints) RETURN d__exp__0, COUNT {s__exp__0}, x__exp__0, SUM {y__exp__0.spec.replicas}, d__exp__1, COUNT {s__exp__1}, x__exp__1, SUM {y__exp__1.spec.replicas}`,
			expectedError: false,
		},
		{
			name:          "No potential kinds found",
			query:         "MATCH (d:Deployment)->(x) RETURN d, x",
			mockKinds:     map[string][]string{},
			expectedError: true,
			errorContains: "unable to determine kind for nodes in relationship",
		},
		{
			name:          "No relationships for kindless node",
			query:         "MATCH (x) RETURN x",
			mockKinds:     map[string][]string{},
			expectedError: true,
			errorContains: "kindless nodes may only be used in a relationship",
		},
		{
			name:          "Kindless-to-kindless relationship",
			query:         "MATCH (x)->(y) RETURN x, y",
			mockKinds:     map[string][]string{},
			expectedError: true,
			errorContains: "chaining two unknown nodes (kindless-to-kindless) is not supported",
		},
		{
			name:          "Match/Return with AS aliases",
			query:         `MATCH (d:Deployment)->(x) RETURN d.metadata.name AS deployment_name, x.spec.replicas AS replica_count`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet) RETURN d__exp__0.metadata.name AS deployment_name, x__exp__0.spec.replicas AS replica_count, d__exp__1.metadata.name AS deployment_name, x__exp__1.spec.replicas AS replica_count`,
			expectedError: false,
		},
		{
			name:          "Match/Return with mixed AS aliases and aggregations",
			query:         `MATCH (d:Deployment)->(x) RETURN d.metadata.name AS deployment_name, COUNT {x} AS pod_count`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet) RETURN d__exp__0.metadata.name AS deployment_name, COUNT {x__exp__0} AS pod_count, d__exp__1.metadata.name AS deployment_name, COUNT {x__exp__1} AS pod_count`,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock
			mockFindPotentialKinds = func(relationships []*Relationship) []string {
				resolver := &MockRelationshipResolver{potentialKindsByNode: tt.mockKinds}
				return resolver.FindPotentialKindsIntersection(relationships)
			}

			// Parse the original query
			ast, err := ParseQuery(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse query: %v", err)
			}

			// Create a query executor with mock providermake
			executor, err := NewQueryExecutor(mockProvider)
			if err != nil {
				t.Fatalf("Failed to create query executor: %v", err)
			}

			// Call rewriteQueryForKindlessNodes
			rewrittenAst, err := executor.rewriteQueryForKindlessNodes(ast)

			// Check error expectations
			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// For queries that don't need rewriting
			if tt.expectedQuery == "" {
				if rewrittenAst != nil {
					t.Error("Expected no rewrite, but got a rewritten AST")
				}
				return
			}

			// Parse the expected query for comparison
			expectedAst, err := ParseQuery(tt.expectedQuery)
			if err != nil {
				t.Fatalf("Failed to parse expected query: %v", err)
			}

			// Debug output
			t.Logf("\nTest case: %s\nExpected AST: %+v\nGot AST: %+v", tt.name, expectedAst, rewrittenAst)

			// Compare the ASTs
			if !reflect.DeepEqual(rewrittenAst, expectedAst) {
				// Print more detailed comparison
				if len(rewrittenAst.Clauses) != len(expectedAst.Clauses) {
					t.Errorf("Number of clauses don't match. Expected %d, got %d", len(expectedAst.Clauses), len(rewrittenAst.Clauses))
				}

				for i, clause := range expectedAst.Clauses {
					if i >= len(rewrittenAst.Clauses) {
						t.Errorf("Missing clause at index %d", i)
						continue
					}

					switch c := clause.(type) {
					case *MatchClause:
						if mc, ok := rewrittenAst.Clauses[i].(*MatchClause); ok {
							t.Logf("Match clause comparison:\nExpected: %+v\nGot: %+v", c, mc)
						} else {
							t.Errorf("Expected MatchClause at index %d, got %T", i, rewrittenAst.Clauses[i])
						}
					case *ReturnClause:
						if rc, ok := rewrittenAst.Clauses[i].(*ReturnClause); ok {
							t.Logf("Return clause comparison:\nExpected: %+v\nGot: %+v", c, rc)
						} else {
							t.Errorf("Expected ReturnClause at index %d, got %T", i, rewrittenAst.Clauses[i])
						}
					}
				}
			}
		})
	}
}
