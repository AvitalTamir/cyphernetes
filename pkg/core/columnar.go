package core

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// ColumnarData represents query results in a columnar format optimized for sorting and limiting
type ColumnarData struct {
	Columns         []string        // Column names (field paths)
	Rows            [][]interface{} // Row data (parallel to NodeIds)
	NodeIds         []string        // Node IDs for each row
	PatternMatchIds []int           // Pattern match IDs to group related resources
}

// NewColumnarData creates a new ColumnarData instance
func NewColumnarData() *ColumnarData {
	return &ColumnarData{
		Columns:         []string{},
		Rows:            [][]interface{}{},
		NodeIds:         []string{},
		PatternMatchIds: []int{},
	}
}

// AddRow adds a row of data to the columnar structure with pattern match tracking
func (cd *ColumnarData) AddRow(data map[string]interface{}, nodeId string, patternMatchId int) {
	// If this is the first row, extract column names from data keys
	if len(cd.Columns) == 0 {
		for key := range data {
			cd.Columns = append(cd.Columns, key)
		}
		sort.Strings(cd.Columns)
	}

	// Build row data in column order
	row := make([]interface{}, len(cd.Columns))
	for i, col := range cd.Columns {
		if val, exists := data[col]; exists {
			row[i] = val
		}
	}

	cd.Rows = append(cd.Rows, row)
	cd.NodeIds = append(cd.NodeIds, nodeId)
	cd.PatternMatchIds = append(cd.PatternMatchIds, patternMatchId)
}

// PatternMatch represents a group of related resources that form one pattern match
type PatternMatch struct {
	Id      int
	Rows    []int // Indices into the main Rows slice
	NodeIds []string
	Data    [][]interface{}
}

// GetPatternMatches groups rows by pattern match ID
func (cd *ColumnarData) GetPatternMatches() []PatternMatch {
	patternMap := make(map[int]*PatternMatch)

	for i, patternId := range cd.PatternMatchIds {
		if _, exists := patternMap[patternId]; !exists {
			patternMap[patternId] = &PatternMatch{
				Id:      patternId,
				Rows:    []int{},
				NodeIds: []string{},
				Data:    [][]interface{}{},
			}
		}

		patternMap[patternId].Rows = append(patternMap[patternId].Rows, i)
		patternMap[patternId].NodeIds = append(patternMap[patternId].NodeIds, cd.NodeIds[i])
		patternMap[patternId].Data = append(patternMap[patternId].Data, cd.Rows[i])
	}

	// Convert map to slice and sort by pattern ID for consistent ordering
	var patterns []PatternMatch
	var patternIds []int
	for id := range patternMap {
		patternIds = append(patternIds, id)
	}
	sort.Ints(patternIds)

	for _, id := range patternIds {
		patterns = append(patterns, *patternMap[id])
	}

	return patterns
}

// OrderBy sorts the data based on the provided ORDER BY items
// For pattern matches, it sorts by the first occurrence of each field within each pattern
func (cd *ColumnarData) OrderBy(orderItems []*OrderByItem) error {
	if len(cd.Rows) == 0 {
		return nil
	}

	patterns := cd.GetPatternMatches()

	// Sort patterns based on ORDER BY criteria
	sort.Slice(patterns, func(i, j int) bool {
		for _, orderItem := range orderItems {
			field := orderItem.Field

			// Find the first occurrence of this field in each pattern
			var valueI, valueJ interface{}

			// Look for the field in pattern i
			for rowIdx, row := range patterns[i].Data {
				value := cd.extractFieldValue(field, row, patterns[i].NodeIds[rowIdx])
				if value != nil {
					valueI = value
					break
				}
			}

			// Look for the field in pattern j
			for rowIdx, row := range patterns[j].Data {
				value := cd.extractFieldValue(field, row, patterns[j].NodeIds[rowIdx])
				if value != nil {
					valueJ = value
					break
				}
			}

			cmp := compareOrderValues(valueI, valueJ)
			if cmp != 0 {
				if orderItem.Direction == "DESC" {
					return cmp > 0 // If valueI > valueJ, pattern i should come first (DESC order)
				}
				return cmp < 0 // If valueI < valueJ, pattern i should come first (ASC order)
			}
		}
		return false
	})

	// Rebuild the columnar data from sorted patterns
	cd.Rows = [][]interface{}{}
	cd.NodeIds = []string{}
	cd.PatternMatchIds = []int{}

	for patternIndex, pattern := range patterns {
		for i := range pattern.Rows {
			cd.Rows = append(cd.Rows, pattern.Data[i])
			cd.NodeIds = append(cd.NodeIds, pattern.NodeIds[i])
			// Use the new pattern index as the PatternMatchId to maintain sorted order
			cd.PatternMatchIds = append(cd.PatternMatchIds, patternIndex)
		}
	}

	return nil
}

// Skip removes the first n pattern matches from the data
func (cd *ColumnarData) Skip(n int) {
	if n <= 0 || len(cd.Rows) == 0 {
		return
	}

	patterns := cd.GetPatternMatches()
	if n >= len(patterns) {
		// Skip all patterns
		cd.Rows = [][]interface{}{}
		cd.NodeIds = []string{}
		cd.PatternMatchIds = []int{}
		return
	}

	// Keep patterns starting from index n
	keptPatterns := patterns[n:]

	// Rebuild the columnar data from kept patterns
	cd.Rows = [][]interface{}{}
	cd.NodeIds = []string{}
	cd.PatternMatchIds = []int{}

	for _, pattern := range keptPatterns {
		for i, _ := range pattern.Rows {
			cd.Rows = append(cd.Rows, pattern.Data[i])
			cd.NodeIds = append(cd.NodeIds, pattern.NodeIds[i])
			cd.PatternMatchIds = append(cd.PatternMatchIds, pattern.Id)
		}
	}
}

// Limit keeps only the first n pattern matches in the data
func (cd *ColumnarData) Limit(n int) {
	if n <= 0 {
		cd.Rows = [][]interface{}{}
		cd.NodeIds = []string{}
		cd.PatternMatchIds = []int{}
		return
	}

	patterns := cd.GetPatternMatches()
	if n >= len(patterns) {
		return // Already within limit
	}

	// Keep only the first n patterns
	keptPatterns := patterns[:n]

	// Rebuild the columnar data from kept patterns
	cd.Rows = [][]interface{}{}
	cd.NodeIds = []string{}
	cd.PatternMatchIds = []int{}

	for _, pattern := range keptPatterns {
		for i, _ := range pattern.Rows {
			cd.Rows = append(cd.Rows, pattern.Data[i])
			cd.NodeIds = append(cd.NodeIds, pattern.NodeIds[i])
			cd.PatternMatchIds = append(cd.PatternMatchIds, pattern.Id)
		}
	}
}

// ConvertToQueryResult converts the columnar data back to QueryResult format
func (cd *ColumnarData) ConvertToQueryResult() map[string]interface{} {
	result := make(map[string]interface{})

	// Group data by node ID
	nodeData := make(map[string][]interface{})

	for i, nodeId := range cd.NodeIds {
		if nodeId == "aggregate" {
			// Handle aggregate data specially
			aggregateData := make(map[string]interface{})
			for j, col := range cd.Columns {
				if j < len(cd.Rows[i]) {
					aggregateData[col] = cd.Rows[i][j]
				}
			}
			result["aggregate"] = aggregateData
			continue
		}

		// Convert row back to map format
		rowData := make(map[string]interface{})
		for j, col := range cd.Columns {
			if j < len(cd.Rows[i]) {
				rowData[col] = cd.Rows[i][j]
			}
		}

		if _, exists := nodeData[nodeId]; !exists {
			nodeData[nodeId] = []interface{}{}
		}
		nodeData[nodeId] = append(nodeData[nodeId], rowData)
	}

	// Add node data to result
	for nodeId, data := range nodeData {
		result[nodeId] = data
	}

	return result
}

// extractFieldValue extracts a field value from a row, handling both simple column names and JSON paths
func (cd *ColumnarData) extractFieldValue(field string, row []interface{}, nodeId string) interface{} {
	// First try exact column match (for aliases)
	for colIdx, colName := range cd.Columns {
		if colName == field && colIdx < len(row) {
			return row[colIdx]
		}
	}

	// If no exact match, check if this is a JSON path that should match one of our columns
	// Build a map from the row data and navigate using the JSON path
	dataMap := make(map[string]interface{})
	for i, col := range cd.Columns {
		if i < len(row) {
			dataMap[col] = row[i]
		}
	}

	if len(field) > 2 && field[1] == '.' {
		// Extract node prefix (e.g., "d" from "d.metadata.name")
		nodePrefix := field[:1]
		path := field[2:] // Remove "d."

		// Check if this row corresponds to the right node
		if nodeId == nodePrefix {
			// Navigate the path in the dataMap
			return navigateJSONPath(dataMap, path)
		}
	}

	return nil
}

// navigateJSONPath navigates a JSON path within a data structure
func navigateJSONPath(data map[string]interface{}, path string) interface{} {
	if path == "" {
		return data
	}

	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if current == nil {
			return nil
		}

		if i == len(parts)-1 {
			// Last part, return the value
			if val, exists := current[part]; exists {
				return val
			}
			return nil
		}

		// Navigate deeper
		if val, exists := current[part]; exists {
			if nextMap, ok := val.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return nil
			}
		} else {
			return nil
		}
	}

	return nil
}

// compareOrderValues compares two values and returns -1, 0, or 1
func compareOrderValues(a, b interface{}) int {
	// Handle nil values
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	// Convert to comparable types
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)

	// Handle same types
	if va.Type() == vb.Type() {
		switch va.Kind() {
		case reflect.String:
			strA := va.String()
			strB := vb.String()
			if strA < strB {
				return -1
			} else if strA > strB {
				return 1
			}
			return 0

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intA := va.Int()
			intB := vb.Int()
			if intA < intB {
				return -1
			} else if intA > intB {
				return 1
			}
			return 0

		case reflect.Float32, reflect.Float64:
			floatA := va.Float()
			floatB := vb.Float()
			if floatA < floatB {
				return -1
			} else if floatA > floatB {
				return 1
			}
			return 0

		case reflect.Bool:
			boolA := va.Bool()
			boolB := vb.Bool()
			if !boolA && boolB {
				return -1
			} else if boolA && !boolB {
				return 1
			}
			return 0
		}
	}

	// Try to convert to strings for comparison
	strA := fmt.Sprintf("%v", a)
	strB := fmt.Sprintf("%v", b)

	// Try to parse as numbers first
	if numA, errA := strconv.ParseFloat(strA, 64); errA == nil {
		if numB, errB := strconv.ParseFloat(strB, 64); errB == nil {
			if numA < numB {
				return -1
			} else if numA > numB {
				return 1
			}
			return 0
		}
	}

	// Fall back to string comparison
	if strA < strB {
		return -1
	} else if strA > strB {
		return 1
	}
	return 0
}
