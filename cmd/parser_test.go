package cmd

import (
	"testing"
)

func TestParseMatchQuery(t *testing.T) {
	// The query to test
	query := "MATCH (k:Kind)"

	// Call the parser
	expr, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	// Check if the result is nil
	if expr == nil {
		t.Fatal("Resulting expression is nil")
	}

	// Check if the result has the expected structure
	if len(expr.Clauses) != 1 {
		t.Fatalf("Expected 1 clause, got %d", len(expr.Clauses))
	}

	// Type assert the first clause to a *MatchClause
	matchClause, ok := expr.Clauses[0].(*MatchClause)
	if !ok {
		t.Fatal("First clause is not a MatchClause")
	}

	// Check the contents of the NodePattern
	if matchClause.NodePattern.Name != "k" || matchClause.NodePattern.Kind != "Kind" {
		t.Errorf("Expected NodePattern with Name 'k' and Kind 'Kind', got Name '%s' and Kind '%s'", matchClause.NodePattern.Name, matchClause.NodePattern.Kind)
	}
}
