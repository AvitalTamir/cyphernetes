package core

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// TabularResult represents data in a table format with columns and rows
type TabularResult struct {
	Columns []string                 // Column names
	Rows    []map[string]interface{} // Each row is a map of column name to value
	NodeMap map[string][]int         // Maps node IDs to their row indices for reconstruction
}

// NewTabularResult creates a new TabularResult
func NewTabularResult() *TabularResult {
	return &TabularResult{
		Columns: make([]string, 0),
		Rows:    make([]map[string]interface{}, 0),
		NodeMap: make(map[string][]int),
	}
}

// DocumentToTabular converts the QueryResult document format to a tabular format
func DocumentToTabular(result *QueryResult, returnClause *ReturnClause) (*TabularResult, error) {
	tabular := NewTabularResult()

	fmt.Printf("Converting to tabular format. Input result: %+v\n", result)
	fmt.Printf("Return clause: %+v\n", returnClause)

	// First, collect all columns from return items
	for _, item := range returnClause.Items {
		colName := item.Alias
		if colName == "" {
			colName = item.JsonPath
		}
		tabular.Columns = append(tabular.Columns, colName)
	}

	fmt.Printf("Collected columns: %v\n", tabular.Columns)

	// Create rows from the document data
	nodeIds := make([]string, 0)
	for nodeId := range result.Data {
		if nodeId != "aggregate" { // Skip aggregate results
			nodeIds = append(nodeIds, nodeId)
		}
	}

	fmt.Printf("Found node IDs: %v\n", nodeIds)

	// For each node's data array
	for _, nodeId := range nodeIds {
		dataArray, ok := result.Data[nodeId].([]interface{})
		if !ok {
			fmt.Printf("Warning: Data for nodeId %s is not an array: %v\n", nodeId, result.Data[nodeId])
			continue
		}

		fmt.Printf("Processing data array for node %s: %+v\n", nodeId, dataArray)

		// For each item in the data array
		for _, data := range dataArray {
			dataMap, ok := data.(map[string]interface{})
			if !ok {
				fmt.Printf("Warning: Data item is not a map: %v\n", data)
				continue
			}

			// Create a new row
			row := make(map[string]interface{})
			for _, item := range returnClause.Items {
				colName := item.Alias
				if colName == "" {
					colName = item.JsonPath
				}

				// If this item belongs to the current node
				if strings.HasPrefix(item.JsonPath, nodeId+".") {
					// Get the nested value using the path
					value := getNestedValue(dataMap, strings.TrimPrefix(item.JsonPath, nodeId+"."))
					row[colName] = value
					fmt.Printf("Added value to row: column=%s value=%v\n", colName, value)
				}
			}

			// Add the row and track its index for the node
			tabular.Rows = append(tabular.Rows, row)
			if tabular.NodeMap[nodeId] == nil {
				tabular.NodeMap[nodeId] = make([]int, 0)
			}
			tabular.NodeMap[nodeId] = append(tabular.NodeMap[nodeId], len(tabular.Rows)-1)
		}
	}

	fmt.Printf("Final tabular result: Columns=%v Rows=%+v NodeMap=%v\n",
		tabular.Columns, tabular.Rows, tabular.NodeMap)

	return tabular, nil
}

// TabularToDocument converts the tabular format back to QueryResult document format
func TabularToDocument(tabular *TabularResult) *QueryResult {
	result := &QueryResult{
		Data:  make(map[string]interface{}),
		Graph: Graph{},
	}

	// For each node, reconstruct its data array
	for nodeId, rowIndices := range tabular.NodeMap {
		nodeData := make([]interface{}, 0)

		// For each row index associated with this node
		for _, idx := range rowIndices {
			if idx >= len(tabular.Rows) {
				continue
			}

			// Create a new data map for this row
			dataMap := make(map[string]interface{})
			row := tabular.Rows[idx]

			// For each column in the row
			for colName, value := range row {
				// If this column belongs to this node
				if strings.HasPrefix(colName, nodeId+".") {
					// Reconstruct the nested structure
					setNestedValue(dataMap, strings.TrimPrefix(colName, nodeId+"."), value)
				}
			}

			nodeData = append(nodeData, dataMap)
		}

		result.Data[nodeId] = nodeData
	}

	return result
}

// ApplyOrderBy sorts the rows based on the ORDER BY clause
func (t *TabularResult) ApplyOrderBy(orderBy []*OrderByItem) error {
	if len(orderBy) == 0 {
		return nil
	}

	fmt.Printf("\n=== ApplyOrderBy ===\n")
	fmt.Printf("Initial rows before sorting: %+v\n", t.Rows)
	fmt.Printf("OrderBy items details:\n")
	for i, item := range orderBy {
		fmt.Printf("  Item %d: JsonPath=%s Desc=%v\n", i, item.JsonPath, item.Desc)
	}
	fmt.Printf("Columns in tabular result: %v\n", t.Columns)
	fmt.Printf("NodeMap: %v\n", t.NodeMap)

	// Validate that all ORDER BY fields exist in at least one row
	for _, item := range orderBy {
		fieldExists := false
		for _, row := range t.Rows {
			if value := row[item.JsonPath]; value != nil {
				fieldExists = true
				break
			}
		}
		if !fieldExists {
			return fmt.Errorf("undefined variable in ORDER BY: %s", item.JsonPath)
		}
	}

	sort.SliceStable(t.Rows, func(i, j int) bool {
		for _, item := range orderBy {
			colName := item.JsonPath
			v1 := t.Rows[i][colName]
			v2 := t.Rows[j][colName]

			fmt.Printf("Comparing rows[%d][%s]=%v with rows[%d][%s]=%v (DESC=%v)\n",
				i, colName, v1, j, colName, v2, item.Desc)

			// Handle nil values
			if v1 == nil && v2 == nil {
				fmt.Printf("Both values nil, continuing to next orderBy item\n")
				continue
			}
			if v1 == nil {
				fmt.Printf("v1 is nil, returning %v\n", !item.Desc)
				return !item.Desc // nil values go first in ASC, last in DESC
			}
			if v2 == nil {
				fmt.Printf("v2 is nil, returning %v\n", item.Desc)
				return item.Desc // nil values go first in ASC, last in DESC
			}

			// Compare values based on their type
			cmp := compareForSort(v1, v2)
			fmt.Printf("compareForSort result: %d\n", cmp)

			if cmp != 0 {
				result := false
				if item.Desc {
					result = cmp > 0 // For DESC order, we want larger values first
				} else {
					result = cmp < 0 // For ASC order, we want smaller values first
				}
				fmt.Printf("Comparison result: %v\n", result)
				return result
			}
		}
		return false
	})

	fmt.Printf("Final rows after sorting: %+v\n", t.Rows)

	// Update NodeMap indices after sorting
	for nodeId := range t.NodeMap {
		t.NodeMap[nodeId] = make([]int, 0)
		for i, row := range t.Rows {
			// Check if this row belongs to this node
			for _, col := range t.Columns {
				if strings.HasPrefix(col, nodeId+".") {
					if _, exists := row[col]; exists {
						t.NodeMap[nodeId] = append(t.NodeMap[nodeId], i)
						break
					}
				}
			}
		}
	}

	return nil
}

// ApplyLimitAndSkip applies LIMIT and SKIP operations to the rows
func (t *TabularResult) ApplyLimitAndSkip(limit, skip int) {
	if skip > 0 {
		if skip >= len(t.Rows) {
			t.Rows = nil
			t.NodeMap = make(map[string][]int)
			return
		}
		t.Rows = t.Rows[skip:]
		// Update NodeMap indices
		for nodeId := range t.NodeMap {
			newIndices := make([]int, 0)
			for _, idx := range t.NodeMap[nodeId] {
				if idx >= skip {
					newIndices = append(newIndices, idx-skip)
				}
			}
			t.NodeMap[nodeId] = newIndices
		}
	}

	if limit > 0 && limit < len(t.Rows) {
		t.Rows = t.Rows[:limit]
		// Update NodeMap indices
		for nodeId := range t.NodeMap {
			newIndices := make([]int, 0)
			for _, idx := range t.NodeMap[nodeId] {
				if idx < limit {
					newIndices = append(newIndices, idx)
				}
			}
			t.NodeMap[nodeId] = newIndices
		}
	}
}

// Helper function to get a nested value from a map using a dot-separated path
func getNestedValue(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if current == nil {
			return nil
		}

		if i == len(parts)-1 {
			return current[part]
		}

		next, ok := current[part].(map[string]interface{})
		if !ok {
			return nil
		}
		current = next
	}

	return nil
}

// Helper function to set a nested value in a map using a dot-separated path
func setNestedValue(data map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}

		if _, exists := current[part]; !exists {
			current[part] = make(map[string]interface{})
		}
		current = current[part].(map[string]interface{})
	}
}

// Helper function to compare two values for sorting
func compareForSort(v1, v2 interface{}) int {
	// Get the types of both values
	t1 := reflect.TypeOf(v1)
	t2 := reflect.TypeOf(v2)

	fmt.Printf("compareForSort: comparing %v (%T) with %v (%T)\n", v1, v1, v2, v2)

	// If types are different, convert to string and compare
	if t1 != t2 {
		s1 := fmt.Sprintf("%v", v1)
		s2 := fmt.Sprintf("%v", v2)
		fmt.Printf("Different types, comparing as strings: '%s' vs '%s'\n", s1, s2)
		if s1 < s2 {
			return -1
		}
		if s1 > s2 {
			return 1
		}
		return 0
	}

	// Compare based on type
	switch v1.(type) {
	case string:
		s1 := v1.(string)
		s2 := v2.(string)
		fmt.Printf("Comparing strings: '%s' vs '%s'\n", s1, s2)
		if s1 < s2 {
			return -1
		}
		if s1 > s2 {
			return 1
		}
		return 0
	case int:
		i1 := v1.(int)
		i2 := v2.(int)
		fmt.Printf("Comparing ints: %d vs %d\n", i1, i2)
		if i1 < i2 {
			return -1
		}
		if i1 > i2 {
			return 1
		}
		return 0
	case float64:
		f1 := v1.(float64)
		f2 := v2.(float64)
		fmt.Printf("Comparing floats: %f vs %f\n", f1, f2)
		if f1 < f2 {
			return -1
		}
		if f1 > f2 {
			return 1
		}
		return 0
	default:
		// For unsupported types, convert to string and compare
		s1 := fmt.Sprintf("%v", v1)
		s2 := fmt.Sprintf("%v", v2)
		fmt.Printf("Comparing unsupported types as strings: '%s' vs '%s'\n", s1, s2)
		if s1 < s2 {
			return -1
		}
		if s1 > s2 {
			return 1
		}
		return 0
	}
}
