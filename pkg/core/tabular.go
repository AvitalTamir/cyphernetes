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
	Graph   Graph                    // Preserve the graph structure during conversion
}

// NewTabularResult creates a new TabularResult
func NewTabularResult() *TabularResult {
	return &TabularResult{
		Columns: make([]string, 0),
		Rows:    make([]map[string]interface{}, 0),
		NodeMap: make(map[string][]int),
		Graph: Graph{
			Nodes: make([]Node, 0),
			Edges: make([]Edge, 0),
		},
	}
}

// DocumentToTabular converts the QueryResult document format to a tabular format
func DocumentToTabular(result *QueryResult, returnClause *ReturnClause) (*TabularResult, error) {
	tabular := NewTabularResult()
	tabular.Graph = result.Graph

	// First, collect all columns from return items
	for _, item := range returnClause.Items {
		colName := item.Alias
		if colName == "" {
			colName = item.JsonPath
		}
		tabular.Columns = append(tabular.Columns, colName)
	}

	// If we have edges, construct rows based on relationships
	if len(result.Graph.Edges) > 0 {
		for _, edge := range result.Graph.Edges {
			// Extract node IDs from edge From/To fields by removing resource type prefix
			fromId := strings.Split(edge.From, "/")[1]
			toId := strings.Split(edge.To, "/")[1]

			// Map these back to the variable names in the query
			fromVarId := ""
			toVarId := ""
			for nodeId := range result.Data {
				// Check if this node's data contains the resource name
				dataArray, ok := result.Data[nodeId].([]interface{})
				if !ok {
					continue
				}
				for _, data := range dataArray {
					dataMap, ok := data.(map[string]interface{})
					if !ok {
						continue
					}
					name, ok := dataMap["name"].(string)
					if !ok {
						continue
					}
					if name == fromId {
						fromVarId = nodeId
					}
					if name == toId {
						toVarId = nodeId
					}
				}
			}

			if fromVarId == "" || toVarId == "" {
				continue
			}

			fromData, fromOk := result.Data[fromVarId].([]interface{})
			toData, toOk := result.Data[toVarId].([]interface{})

			if !fromOk || !toOk {
				continue
			}

			// For each combination of from/to data, create a row
			for _, fromItem := range fromData {
				fromMap, ok := fromItem.(map[string]interface{})
				if !ok {
					continue
				}
				if fromMap["name"].(string) != fromId {
					continue
				}

				for _, toItem := range toData {
					toMap, ok := toItem.(map[string]interface{})
					if !ok {
						continue
					}
					if toMap["name"].(string) != toId {
						continue
					}

					// Create a new row combining both resources
					row := make(map[string]interface{})

					// Add special name columns for each node
					row[fromVarId+".__name"] = fromId
					row[toVarId+".__name"] = toId

					for _, item := range returnClause.Items {
						colName := item.Alias
						if colName == "" {
							colName = item.JsonPath
						}

						// Check which resource this column belongs to
						if strings.HasPrefix(item.JsonPath, fromVarId+".") {
							value := getNestedValue(fromMap, strings.TrimPrefix(item.JsonPath, fromVarId+"."))
							row[colName] = value
						} else if strings.HasPrefix(item.JsonPath, toVarId+".") {
							value := getNestedValue(toMap, strings.TrimPrefix(item.JsonPath, toVarId+"."))
							row[colName] = value
						}
					}

					// Add the row and track indices for both nodes
					tabular.Rows = append(tabular.Rows, row)
					rowIdx := len(tabular.Rows) - 1

					if tabular.NodeMap[fromVarId] == nil {
						tabular.NodeMap[fromVarId] = make([]int, 0)
					}
					if tabular.NodeMap[toVarId] == nil {
						tabular.NodeMap[toVarId] = make([]int, 0)
					}
					tabular.NodeMap[fromVarId] = append(tabular.NodeMap[fromVarId], rowIdx)
					tabular.NodeMap[toVarId] = append(tabular.NodeMap[toVarId], rowIdx)
				}
			}
		}
	} else {
		// No edges, fall back to node-based rows
		nodeIds := make([]string, 0)
		for nodeId := range result.Data {
			if nodeId != "aggregate" {
				nodeIds = append(nodeIds, nodeId)
			}
		}

		// For each node's data array
		for _, nodeId := range nodeIds {
			dataArray, ok := result.Data[nodeId].([]interface{})
			if !ok {
				continue
			}

			for _, data := range dataArray {
				dataMap, ok := data.(map[string]interface{})
				if !ok {
					continue
				}

				row := make(map[string]interface{})

				// Add special name column
				if name, ok := dataMap["name"].(string); ok {
					row[nodeId+".__name"] = name
				}

				for _, item := range returnClause.Items {
					colName := item.Alias
					if colName == "" {
						colName = item.JsonPath
					}

					if strings.HasPrefix(item.JsonPath, nodeId+".") {
						value := getNestedValue(dataMap, strings.TrimPrefix(item.JsonPath, nodeId+"."))
						row[colName] = value
					}
				}

				tabular.Rows = append(tabular.Rows, row)
				if tabular.NodeMap[nodeId] == nil {
					tabular.NodeMap[nodeId] = make([]int, 0)
				}
				tabular.NodeMap[nodeId] = append(tabular.NodeMap[nodeId], len(tabular.Rows)-1)
			}
		}
	}

	return tabular, nil
}

// TabularToDocument converts the tabular format back to QueryResult document format
func TabularToDocument(tabular *TabularResult) *QueryResult {
	result := &QueryResult{
		Data:  make(map[string]interface{}),
		Graph: tabular.Graph, // Use the filtered graph that was updated in ApplyLimitAndSkip
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

			// Add the special "name" property from our preserved value
			if name, ok := row[nodeId+".__name"].(string); ok {
				dataMap["name"] = name
			}

			// For each column in the row
			for colName, value := range row {
				// If this column belongs to this node and isn't our special name field
				if strings.HasPrefix(colName, nodeId+".") && !strings.HasSuffix(colName, ".__name") {
					// Reconstruct the nested structure
					setNestedValue(dataMap, strings.TrimPrefix(colName, nodeId+"."), value)
				}
			}

			if len(dataMap) > 0 {
				nodeData = append(nodeData, dataMap)
			}
		}

		if len(nodeData) > 0 {
			result.Data[nodeId] = nodeData
		}
	}

	return result
}

// ApplyOrderBy sorts the rows based on the ORDER BY clause
func (t *TabularResult) ApplyOrderBy(orderBy []*OrderByItem) error {
	if len(orderBy) == 0 {
		return nil
	}

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

			// Handle nil values
			if v1 == nil && v2 == nil {
				continue
			}
			if v1 == nil {
				return !item.Desc // nil values go first in ASC, last in DESC
			}
			if v2 == nil {
				return item.Desc // nil values go first in ASC, last in DESC
			}

			// Compare values based on their type
			cmp := compareForSort(v1, v2)

			if cmp != 0 {
				result := false
				if item.Desc {
					result = cmp > 0 // For DESC order, we want larger values first
				} else {
					result = cmp < 0 // For ASC order, we want smaller values first
				}
				return result
			}
		}
		return false
	})

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
			t.Graph = Graph{
				Nodes: make([]Node, 0),
				Edges: make([]Edge, 0),
			}
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
			if len(newIndices) > 0 {
				t.NodeMap[nodeId] = newIndices
			} else {
				delete(t.NodeMap, nodeId)
			}
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
			if len(newIndices) > 0 {
				t.NodeMap[nodeId] = newIndices
			} else {
				delete(t.NodeMap, nodeId)
			}
		}
	}

	// Build maps of remaining resources and their types from the rows
	remainingResources := make(map[string]bool) // Just the name
	resourceTypes := make(map[string]string)    // name -> type

	// First, build a map of resource names to their types from the graph nodes
	nodeTypes := make(map[string]string) // name -> Kind
	for _, node := range t.Graph.Nodes {
		nodeTypes[node.Name] = node.Kind
	}

	// Then collect remaining resources from the rows
	for _, row := range t.Rows {
		for colName, value := range row {
			if strings.HasSuffix(colName, ".name") && value != nil {
				name := value.(string)
				remainingResources[name] = true
				// Use the actual Kind from the graph nodes
				if kind, exists := nodeTypes[name]; exists {
					resourceTypes[name] = kind
				}
			}
		}
	}

	// Filter nodes - keep only those referenced in remaining rows
	filteredNodes := make([]Node, 0)
	for _, node := range t.Graph.Nodes {
		if remainingResources[node.Name] {
			filteredNodes = append(filteredNodes, node)
		}
	}
	t.Graph.Nodes = filteredNodes

	// Filter edges - only keep edges where both nodes still exist
	filteredEdges := make([]Edge, 0)
	for _, edge := range t.Graph.Edges {
		fromParts := strings.Split(edge.From, "/")
		toParts := strings.Split(edge.To, "/")
		if len(fromParts) != 2 || len(toParts) != 2 {
			continue
		}

		fromName := fromParts[1]
		toName := toParts[1]

		// Check if both resources exist and their types match
		if remainingResources[fromName] && remainingResources[toName] {
			// Verify that the resource types match what we found in the graph
			if fromType, exists := resourceTypes[fromName]; exists && fromParts[0] == fromType {
				if toType, exists := resourceTypes[toName]; exists && toParts[0] == toType {
					filteredEdges = append(filteredEdges, edge)
				}
			}
		}
	}
	t.Graph.Edges = filteredEdges
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

	// If types are different, convert to string and compare
	if t1 != t2 {
		s1 := fmt.Sprintf("%v", v1)
		s2 := fmt.Sprintf("%v", v2)
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
		if s1 < s2 {
			return -1
		}
		if s1 > s2 {
			return 1
		}
		return 0
	}
}
