package core

import (
	"fmt"
	"strings"

	"github.com/AvitalTamir/jsonpath"
)

func matchByCriteria(resourceA, resourceB interface{}, criteria []MatchCriterion) bool {
	for _, criterion := range criteria {
		switch criterion.ComparisonType {
		case ContainsAll:
			l, err := jsonpath.JsonPathLookup(resourceA, strings.ReplaceAll(criterion.FieldA, "[]", ""))
			if err != nil {
				return false
			}
			labels, ok := l.(map[string]interface{})
			if !ok {
				return false
			}

			s, err := jsonpath.JsonPathLookup(resourceB, strings.ReplaceAll(criterion.FieldB, "[]", ""))
			if err != nil {
				return false
			}
			selector, ok := s.(map[string]interface{})
			if !ok {
				return false
			}

			if !matchContainsAll(labels, selector) {
				return false
			}

		case ExactMatch:
			// Extract the fields
			fieldsA, err := jsonpath.JsonPathLookup(resourceA, strings.ReplaceAll(criterion.FieldA, "[]", ""))
			if err != nil {
				return false
			}
			fieldsB, err := jsonpath.JsonPathLookup(resourceB, strings.ReplaceAll(criterion.FieldB, "[]", ""))
			if err != nil {
				return false
			}
			if !matchFields(fieldsA, fieldsB) {
				return false
			}

		case StringContains:
			// Extract the fields
			fieldA, err := jsonpath.JsonPathLookup(resourceA, strings.ReplaceAll(criterion.FieldA, "[]", ""))
			if err != nil {
				return false
			}
			fieldB, err := jsonpath.JsonPathLookup(resourceB, strings.ReplaceAll(criterion.FieldB, "[]", ""))
			if err != nil {
				return false
			}

			// Convert both fields to strings
			strA := fmt.Sprintf("%v", fieldA)
			strB := fmt.Sprintf("%v", fieldB)

			// Check if fieldA contains fieldB
			if !strings.Contains(strA, strB) {
				return false
			}
		}
	}
	return true
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

	// if fieldA is a slice, iterate over it and compare each element to fieldB
	fieldASlice, ok := fieldA.([]interface{})
	if ok {
		for _, element := range fieldASlice {
			if matchFields(element, fieldB) {
				return true
			}
		}
		return false
	}

	// if fieldA is a map, iterate over it and compare each value to fieldB
	fieldAMap, ok := fieldA.(map[string]interface{})
	if ok {
		for _, value := range fieldAMap {
			if matchFields(value, fieldB) {
				return true
			}
		}
		return false
	}

	// if fieldA is a number, compare it to fieldB
	fieldANumber, ok := fieldA.(float64)
	if ok {
		fieldBNumber, ok := fieldB.(float64)
		if ok {
			return fieldANumber == fieldBNumber
		}
		return false
	}

	// if fieldA is a bool, compare it to fieldB
	fieldABool, ok := fieldA.(bool)
	if ok {
		fieldBBool, ok := fieldB.(bool)
		if ok {
			return fieldABool == fieldBBool
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
