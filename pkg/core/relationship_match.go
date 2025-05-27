package core

import (
	"fmt"
	"strings"
)

func matchByCriterion(resourceA, resourceB interface{}, criterion MatchCriterion) bool {
	switch criterion.ComparisonType {
	case ContainsAll:
		l, err := JsonPathCompileAndLookup(resourceA, strings.ReplaceAll(criterion.FieldA, "[]", ""))
		if err != nil {
			return false
		}
		labels, ok := l.(map[string]interface{})
		if !ok {
			return false
		}

		s, err := JsonPathCompileAndLookup(resourceB, strings.ReplaceAll(criterion.FieldB, "[]", ""))
		if err != nil {
			return false
		}
		selector, ok := s.(map[string]interface{})
		if !ok {
			return false
		}

		return matchContainsAll(labels, selector)

	case ExactMatch:
		// Extract the fields
		fieldsA, err := JsonPathCompileAndLookup(resourceA, strings.ReplaceAll(criterion.FieldA, "[]", ""))
		if err != nil {
			return false
		}
		fieldsB, err := JsonPathCompileAndLookup(resourceB, strings.ReplaceAll(criterion.FieldB, "[]", ""))
		if err != nil {
			return false
		}
		return matchFields(fieldsA, fieldsB)

	case StringContains:
		// Extract the fields
		fieldA, err := JsonPathCompileAndLookup(resourceA, strings.ReplaceAll(criterion.FieldA, "[]", ""))
		if err != nil {
			return false
		}
		fieldB, err := JsonPathCompileAndLookup(resourceB, strings.ReplaceAll(criterion.FieldB, "[]", ""))
		if err != nil {
			return false
		}

		// Convert both fields to strings
		strA := fmt.Sprintf("%v", fieldA)
		strB := fmt.Sprintf("%v", fieldB)

		// Check if fieldA contains fieldB
		return strings.Contains(strA, strB)
	}
	return false
}

func matchFields(fieldA, fieldB interface{}) bool {
	// if fieldA is a string, compare it to fieldB
	fieldAString, ok := fieldA.(string)
	if ok {
		fieldBString, ok := fieldB.(string)
		if ok {
			return fieldAString == fieldBString
		}
		return false
	}

	// if fieldA is a slice, flatten it and compare each element to fieldB
	fieldASlice, ok := fieldA.([]interface{})
	if ok {
		// Flatten nested slices
		var flatSlice []interface{}
		for _, element := range fieldASlice {
			if nestedSlice, isSlice := element.([]interface{}); isSlice {
				flatSlice = append(flatSlice, nestedSlice...)
			} else {
				flatSlice = append(flatSlice, element)
			}
		}

		// Compare each element in the flattened slice
		for _, element := range flatSlice {
			if matchFields(element, fieldB) {
				return true
			}
		}
		return false
	}

	// if fieldA is a map, traverse it recursively
	fieldAMap, ok := fieldA.(map[string]interface{})
	if ok {
		for _, value := range fieldAMap {
			if matchFields(value, fieldB) {
				return true
			}
		}
		return false
	}

	// if fieldA is nil, compare to fieldB nil
	if fieldA == nil {
		return fieldB == nil
	}

	// Direct comparison as last resort
	return fieldA == fieldB
}

func matchContainsAll(labels, selector map[string]interface{}) bool {
	if len(selector) == 0 || len(labels) == 0 {
		return false
	}
	// validate all labels in the selector exist on the labels and match
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}
