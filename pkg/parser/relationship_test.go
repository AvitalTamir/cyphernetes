package parser

import (
	"reflect"
	"testing"
)

func TestFindRuleByRelationshipType(t *testing.T) {
	tests := []struct {
		name             string
		relationshipType RelationshipType
		expectedRule     RelationshipRule
		expectError      bool
	}{
		{
			name:             "Valid relationship type",
			relationshipType: ServiceExposePod,
			expectedRule: RelationshipRule{
				KindA:        "pods",
				KindB:        "services",
				Relationship: ServiceExposePod,
				MatchCriteria: []MatchCriterion{{
					FieldA:         "$.metadata.labels",
					FieldB:         "$.spec.selector",
					ComparisonType: ContainsAll,
					DefaultProps: []DefaultProp{
						{
							FieldA:  "",
							FieldB:  "$.spec.ports[].port",
							Default: 80,
						},
					},
				}},
			},
			expectError: false,
		},
		{
			name:             "Invalid relationship type",
			relationshipType: "INVALID",
			expectedRule:     RelationshipRule{},
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := findRuleByRelationshipType(tt.relationshipType)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(rule, tt.expectedRule) {
					t.Errorf("Expected rule %+v, but got %+v", tt.expectedRule, rule)
				}
			}
		})
	}
}

func TestMatchByCriteria(t *testing.T) {
	tests := []struct {
		name        string
		resourceA   interface{}
		resourceB   interface{}
		criteria    []MatchCriterion
		expectMatch bool
	}{
		{
			name: "Matching resources",
			resourceA: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": "example",
					},
				},
			},
			resourceB: map[string]interface{}{
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "example",
						},
					},
				},
			},
			criteria: []MatchCriterion{
				{FieldA: "$.metadata.labels", FieldB: "$.spec.selector.matchLabels", ComparisonType: ContainsAll},
			},
			expectMatch: true,
		},
		{
			name: "Non-matching resources",
			resourceA: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": "example1",
					},
				},
			},
			resourceB: map[string]interface{}{
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "example2",
						},
					},
				},
			},
			criteria: []MatchCriterion{
				{FieldA: "$.metadata.labels", FieldB: "$.spec.selector.matchLabels", ComparisonType: ContainsAll},
			},
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchByCriteria(tt.resourceA, tt.resourceB, tt.criteria)
			if result != tt.expectMatch {
				t.Errorf("Expected match: %v, but got: %v", tt.expectMatch, result)
			}
		})
	}
}

func TestMatchFields(t *testing.T) {
	tests := []struct {
		name        string
		fieldA      interface{}
		fieldB      interface{}
		expectMatch bool
	}{
		{
			name:        "Matching strings",
			fieldA:      "test",
			fieldB:      "test",
			expectMatch: true,
		},
		{
			name:        "Non-matching strings",
			fieldA:      "test1",
			fieldB:      "test2",
			expectMatch: false,
		},
		{
			name:        "Matching numbers",
			fieldA:      float64(42),
			fieldB:      float64(42),
			expectMatch: true,
		},
		{
			name:        "Non-matching numbers",
			fieldA:      float64(42),
			fieldB:      float64(24),
			expectMatch: false,
		},
		{
			name:        "Matching booleans",
			fieldA:      true,
			fieldB:      true,
			expectMatch: true,
		},
		{
			name:        "Non-matching booleans",
			fieldA:      true,
			fieldB:      false,
			expectMatch: false,
		},
		{
			name:        "Matching nested map",
			fieldA:      map[string]interface{}{"a": map[string]interface{}{"b": "test"}},
			fieldB:      "test",
			expectMatch: true,
		},
		{
			name:        "Matching nested slice",
			fieldA:      []interface{}{1, "test", map[string]interface{}{"a": "test"}},
			fieldB:      "test",
			expectMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchFields(tt.fieldA, tt.fieldB)
			if result != tt.expectMatch {
				t.Errorf("Expected match: %v, but got: %v", tt.expectMatch, result)
			}
		})
	}
}

func TestMatchContainsAll(t *testing.T) {
	tests := []struct {
		name        string
		labels      map[string]interface{}
		selector    map[string]interface{}
		expectMatch bool
	}{
		{
			name:        "Matching labels",
			labels:      map[string]interface{}{"app": "example", "env": "prod"},
			selector:    map[string]interface{}{"app": "example"},
			expectMatch: true,
		},
		{
			name:        "Non-matching labels",
			labels:      map[string]interface{}{"app": "example", "env": "prod"},
			selector:    map[string]interface{}{"app": "other"},
			expectMatch: false,
		},
		{
			name:        "Empty selector",
			labels:      map[string]interface{}{"app": "example"},
			selector:    map[string]interface{}{},
			expectMatch: false,
		},
		{
			name:        "Empty labels",
			labels:      map[string]interface{}{},
			selector:    map[string]interface{}{"app": "example"},
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchContainsAll(tt.labels, tt.selector)
			if result != tt.expectMatch {
				t.Errorf("Expected match: %v, but got: %v", tt.expectMatch, result)
			}
		})
	}
}

func TestApplyRelationshipRule(t *testing.T) {
	tests := []struct {
		name           string
		resourcesA     []map[string]interface{}
		resourcesB     []map[string]interface{}
		rule           RelationshipRule
		direction      Direction
		expectedResult map[string]interface{}
	}{
		{
			name: "Matching resources with Left direction",
			resourcesA: []map[string]interface{}{
				{"metadata": map[string]interface{}{"name": "resA1", "labels": map[string]interface{}{"app": "example"}}},
				{"metadata": map[string]interface{}{"name": "resA2", "labels": map[string]interface{}{"app": "other"}}},
			},
			resourcesB: []map[string]interface{}{
				{"metadata": map[string]interface{}{"name": "resB1"}, "spec": map[string]interface{}{"selector": map[string]interface{}{"matchLabels": map[string]interface{}{"app": "example"}}}},
			},
			rule: RelationshipRule{
				Relationship:  "CONTAINS",
				MatchCriteria: []MatchCriterion{{FieldA: "$.metadata.labels", FieldB: "$.spec.selector.matchLabels", ComparisonType: ContainsAll}},
			},
			direction: Left,
			expectedResult: map[string]interface{}{
				"right": []map[string]interface{}{
					{"metadata": map[string]interface{}{"name": "resA1", "labels": map[string]interface{}{"app": "example"}}},
				},
				"left": []map[string]interface{}{
					{"metadata": map[string]interface{}{"name": "resB1"}, "spec": map[string]interface{}{"selector": map[string]interface{}{"matchLabels": map[string]interface{}{"app": "example"}}}},
				},
			},
		},
		// Add more test cases for different scenarios and directions
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyRelationshipRule(tt.resourcesA, tt.resourcesB, tt.rule, tt.direction)
			if !reflect.DeepEqual(result, tt.expectedResult) {
				t.Errorf("Expected result: %+v, but got: %+v", tt.expectedResult, result)
			}
		})
	}
}
