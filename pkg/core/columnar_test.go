package core

import (
	"testing"
)

func TestColumnarData(t *testing.T) {
	// Create test data
	cd := NewColumnarData()

	// Add some test rows
	row1 := map[string]interface{}{
		"name":   "pod1",
		"cpu":    "100m",
		"memory": "128Mi",
		"age":    5,
	}
	row2 := map[string]interface{}{
		"name":   "pod2",
		"cpu":    "200m",
		"memory": "256Mi",
		"age":    10,
	}
	row3 := map[string]interface{}{
		"name":   "pod3",
		"cpu":    "50m",
		"memory": "64Mi",
		"age":    2,
	}

	cd.AddRow(row1, "node1", 0)
	cd.AddRow(row2, "node1", 1)
	cd.AddRow(row3, "node2", 2)

	// Test initial state
	if len(cd.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(cd.Rows))
	}

	// Test ORDER BY age ASC
	orderBy := []*OrderByItem{{Field: "age", Direction: "ASC"}}
	err := cd.OrderBy(orderBy)
	if err != nil {
		t.Fatalf("OrderBy failed: %v", err)
	}

	// Convert back to check results
	result := cd.ConvertToQueryResult()

	// Check that we have the right number of results
	node1Data, ok := result["node1"].([]interface{})
	if !ok {
		t.Fatalf("Expected node1 data to be a slice")
	}
	node2Data, ok := result["node2"].([]interface{})
	if !ok {
		t.Fatalf("Expected node2 data to be a slice")
	}

	totalRows := len(node1Data) + len(node2Data)
	if totalRows != 3 {
		t.Errorf("Expected 3 total rows after OrderBy, got %d", totalRows)
	}

	// Test ORDER BY age DESC
	orderByDesc := []*OrderByItem{{Field: "age", Direction: "DESC"}}
	err = cd.OrderBy(orderByDesc)
	if err != nil {
		t.Fatalf("OrderBy DESC failed: %v", err)
	}

	// Verify ordering worked (we'll test this more thoroughly in pattern match tests)
	result = cd.ConvertToQueryResult()
	node1Data, _ = result["node1"].([]interface{})
	node2Data, _ = result["node2"].([]interface{})
	totalRows = len(node1Data) + len(node2Data)
	if totalRows != 3 {
		t.Errorf("Expected 3 total rows after OrderBy DESC, got %d", totalRows)
	}

	// Test LIMIT
	cd.Limit(2)
	if len(cd.Rows) != 2 {
		t.Errorf("Expected 2 rows after LIMIT 2, got %d", len(cd.Rows))
	}

	// Test SKIP
	cd.Skip(1)
	if len(cd.Rows) != 1 {
		t.Errorf("Expected 1 row after SKIP 1, got %d", len(cd.Rows))
	}
}

func TestColumnarDataOrderByString(t *testing.T) {
	cd := NewColumnarData()

	// Add rows with string values to test
	row1 := map[string]interface{}{"name": "zebra", "value": 1}
	row2 := map[string]interface{}{"name": "apple", "value": 2}
	row3 := map[string]interface{}{"name": "banana", "value": 3}

	cd.AddRow(row1, "node1", 0)
	cd.AddRow(row2, "node1", 1)
	cd.AddRow(row3, "node1", 2)

	// Test ORDER BY name ASC (alphabetical)
	orderBy := []*OrderByItem{{Field: "name", Direction: "ASC"}}
	err := cd.OrderBy(orderBy)
	if err != nil {
		t.Fatalf("OrderBy failed: %v", err)
	}

	// Convert back to check results
	result := cd.ConvertToQueryResult()
	node1Data, ok := result["node1"].([]interface{})
	if !ok {
		t.Fatalf("Expected node1 data to be a slice")
	}

	if len(node1Data) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(node1Data))
	}

	// Check order: should be apple, banana, zebra
	expectedOrder := []string{"apple", "banana", "zebra"}
	for i, expected := range expectedOrder {
		if i < len(node1Data) {
			if rowData, ok := node1Data[i].(map[string]interface{}); ok {
				if name, ok := rowData["name"].(string); ok {
					if name != expected {
						t.Errorf("Expected row %d to have name %s, got %s", i, expected, name)
					}
				} else {
					t.Errorf("Expected row %d to have name field", i)
				}
			} else {
				t.Errorf("Expected row %d to be a map", i)
			}
		}
	}
}

func TestColumnarDataMultipleOrderBy(t *testing.T) {
	cd := NewColumnarData()

	// Add rows with same age but different names
	row1 := map[string]interface{}{"name": "pod1", "age": 5, "priority": 1}
	row2 := map[string]interface{}{"name": "pod2", "age": 5, "priority": 2}
	row3 := map[string]interface{}{"name": "pod3", "age": 10, "priority": 1}

	cd.AddRow(row1, "node1", 0)
	cd.AddRow(row2, "node1", 1)
	cd.AddRow(row3, "node1", 2)

	// Test ORDER BY age ASC, then by priority DESC
	orderBy := []*OrderByItem{
		{Field: "age", Direction: "ASC"},
		{Field: "priority", Direction: "DESC"},
	}
	err := cd.OrderBy(orderBy)
	if err != nil {
		t.Fatalf("OrderBy failed: %v", err)
	}

	// Convert back to check results
	result := cd.ConvertToQueryResult()
	node1Data, ok := result["node1"].([]interface{})
	if !ok {
		t.Fatalf("Expected node1 data to be a slice")
	}

	if len(node1Data) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(node1Data))
	}

	// Check order: age 5 first (pod2 with priority 2, then pod1 with priority 1), then age 10 (pod3)
	expectedOrder := []string{"pod2", "pod1", "pod3"}
	for i, expected := range expectedOrder {
		if i < len(node1Data) {
			if rowData, ok := node1Data[i].(map[string]interface{}); ok {
				if name, ok := rowData["name"].(string); ok {
					if name != expected {
						t.Errorf("Expected row %d to have name %s, got %s", i, expected, name)
					}
				} else {
					t.Errorf("Expected row %d to have name field", i)
				}
			} else {
				t.Errorf("Expected row %d to be a map", i)
			}
		}
	}
}

func TestColumnarDataPatternMatches(t *testing.T) {
	cd := NewColumnarData()

	// Simulate pattern matches: 2 deployments, each with 2 pods
	// Pattern 0: deployment-1 with pod-1-1, pod-1-2
	// Pattern 1: deployment-2 with pod-2-1, pod-2-2

	// Pattern 0 resources
	deploy1 := map[string]interface{}{"name": "deployment-1", "replicas": 2}
	pod11 := map[string]interface{}{"name": "pod-1-1", "cpu": "100m"}
	pod12 := map[string]interface{}{"name": "pod-1-2", "cpu": "150m"}

	// Pattern 1 resources
	deploy2 := map[string]interface{}{"name": "deployment-2", "replicas": 1}
	pod21 := map[string]interface{}{"name": "pod-2-1", "cpu": "200m"}
	pod22 := map[string]interface{}{"name": "pod-2-2", "cpu": "250m"}

	// Add resources with pattern match IDs
	cd.AddRow(deploy1, "d", 0)
	cd.AddRow(pod11, "p", 0)
	cd.AddRow(pod12, "p", 0)

	cd.AddRow(deploy2, "d", 1)
	cd.AddRow(pod21, "p", 1)
	cd.AddRow(pod22, "p", 1)

	// Test that we have 6 total rows
	if len(cd.Rows) != 6 {
		t.Errorf("Expected 6 rows, got %d", len(cd.Rows))
	}

	// Test LIMIT 1 - should keep only the first pattern match (3 resources)
	cd.Limit(1)

	// Should have 3 rows left (deployment-1 + 2 pods)
	if len(cd.Rows) != 3 {
		t.Errorf("Expected 3 rows after LIMIT 1, got %d", len(cd.Rows))
	}

	// Convert back and verify we only have pattern 0 resources
	result := cd.ConvertToQueryResult()

	deployments, ok := result["d"].([]interface{})
	if !ok {
		t.Fatalf("Expected deployment data to be a slice")
	}
	pods, ok := result["p"].([]interface{})
	if !ok {
		t.Fatalf("Expected pod data to be a slice")
	}

	// Should have 1 deployment and 2 pods from pattern 0
	if len(deployments) != 1 {
		t.Errorf("Expected 1 deployment after LIMIT 1, got %d", len(deployments))
	}
	if len(pods) != 2 {
		t.Errorf("Expected 2 pods after LIMIT 1, got %d", len(pods))
	}

	// Verify it's the correct deployment
	if deployData, ok := deployments[0].(map[string]interface{}); ok {
		if name, ok := deployData["name"].(string); ok {
			if name != "deployment-1" {
				t.Errorf("Expected deployment-1, got %s", name)
			}
		}
	}
}
