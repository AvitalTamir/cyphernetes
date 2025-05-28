package core

import (
	"strings"

	"github.com/AvitalTamir/jsonpath"
)

func applyWildcardUpdate(resource interface{}, path string, value interface{}) error {
	parts := strings.Split(path, "[*]")
	return applyWildcardUpdateRecursive(resource, parts, 0, value)
}

func applyWildcardUpdateRecursive(data interface{}, parts []string, depth int, value interface{}) error {
	if depth == len(parts)-1 {
		// Last part - set the value
		return setValueAtPath(data, parts[depth], value)
	}

	// Get the array at current level
	currentPath := parts[depth]
	if !strings.HasSuffix(currentPath, ".") {
		currentPath += "."
	}
	array, err := JsonPathCompileAndLookup(data, currentPath)
	if err != nil {
		return err
	}

	// Update all elements in the array
	switch arr := array.(type) {
	case []interface{}:
		for _, item := range arr {
			if err := applyWildcardUpdateRecursive(item, parts, depth+1, value); err != nil {
				return err
			}
		}
	case []map[string]interface{}:
		for _, item := range arr {
			if err := applyWildcardUpdateRecursive(item, parts, depth+1, value); err != nil {
				return err
			}
		}
	}

	return nil
}

func splitEscapedPath(path string) []string {
	var result []string
	var current strings.Builder
	escaped := false

	for i := 0; i < len(path); i++ {
		if escaped {
			// If we're in escaped mode and see a dot, add it to current
			if path[i] == '.' {
				current.WriteByte('.')
			} else {
				// Otherwise, write both the backslash and the current character
				current.WriteByte('\\')
				current.WriteByte(path[i])
			}
			escaped = false
		} else if path[i] == '\\' {
			// Enter escaped mode
			escaped = true
		} else if path[i] == '.' && !escaped {
			// Unescaped dot, split here
			result = append(result, current.String())
			current.Reset()
		} else {
			// Regular character
			current.WriteByte(path[i])
		}
	}

	// Add the last part
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

func evaluateWildcardPath(resource interface{}, path string, filterValue interface{}, operator string) bool {
	// Get the base path (everything before [*])
	basePath := path[:strings.Index(path, "[*]")]
	if !strings.HasPrefix(basePath, "$.") {
		basePath = "$." + basePath
	}

	// Get the array using the base path
	array, err := JsonPathCompileAndLookup(resource, basePath)
	if err != nil {
		return false
	}

	// Convert to array of interfaces
	items, ok := array.([]interface{})
	if !ok {
		return false
	}

	// Get the remaining path after [*]
	remainingPath := path[strings.Index(path, "[*]")+3:]
	if remainingPath != "" && !strings.HasPrefix(remainingPath, ".") {
		remainingPath = "." + remainingPath
	}

	// Check each item in the array
	for _, item := range items {
		// For primitive array items
		if remainingPath == "" {
			itemValue, filterValue, err := convertToComparableTypes(item, filterValue)
			if err != nil {
				continue
			}
			if compareValues(itemValue, filterValue, operator) {
				return true
			}
			continue
		}

		// For object array items
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Create a new path for this item
		itemPath := "$" + remainingPath

		value, err := JsonPathCompileAndLookup(itemMap, itemPath)
		if err != nil {
			continue
		}

		resourceValue, filterValue, err := convertToComparableTypes(value, filterValue)
		if err != nil {
			continue
		}

		if compareValues(resourceValue, filterValue, operator) {
			return true
		}
	}

	return false
}

// fixCompiledPath fixes escape characters in the query.
func fixCompiledPath(compiledPath *jsonpath.Compiled) *jsonpath.Compiled {
	i := 0
	for i < len(compiledPath.Steps) {
		step := compiledPath.Steps[i]
		if strings.HasSuffix(step.Key, "\\") && i+1 < len(compiledPath.Steps) {
			nextStep := compiledPath.Steps[i+1]
			step.Key = step.Key[:len(step.Key)-1] + "." + nextStep.Key
			compiledPath.Steps[i] = step
			compiledPath.Steps = append(compiledPath.Steps[:i+1], compiledPath.Steps[i+2:]...)
		} else {
			i++
		}
	}
	return compiledPath
}

// JsonPathCompileAndLookup compiles the given query, fixes escape characters in the query and executes
// the query to return the value of the query.
func JsonPathCompileAndLookup(resource interface{}, query string) (interface{}, error) {
	pathA, err := jsonpath.Compile(query)
	if err != nil {
		return nil, err
	}
	queriedValue, err := fixCompiledPath(pathA).Lookup(resource)
	if err != nil {
		return nil, err
	}
	return queriedValue, nil
}
