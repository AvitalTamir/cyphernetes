package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
			name: "Single criterion match",
			resourceA: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
			},
			resourceB: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
			},
			criteria: []MatchCriterion{
				{
					FieldA:         "$.metadata.name",
					FieldB:         "$.metadata.name",
					ComparisonType: ExactMatch,
				},
			},
			expectMatch: true,
		},
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
			// Test each criterion individually
			for _, criterion := range tt.criteria {
				result := matchByCriterion(tt.resourceA, tt.resourceB, criterion)
				if result != tt.expectMatch {
					t.Errorf("matchByCriterion() with criterion %+v = %v, want %v",
						criterion, result, tt.expectMatch)
				}
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

func TestLoadCustomRelationships(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create .cyphernetes directory
	cyphernetesDir := filepath.Join(tmpDir, ".cyphernetes")
	if err := os.MkdirAll(cyphernetesDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name               string
		yamlContent        string
		expectedRules      []RelationshipRule
		expectedError      bool
		errorContains      string
		knownResourceKinds []string
	}{
		{
			name: "Valid custom relationships",
			yamlContent: `
relationships:
  - kindA: pods
    kindB: configmaps
    relationship: POD_USE_CONFIGMAP
    matchCriteria:
      - fieldA: "$.spec.volumes[].configMap.name"
        fieldB: "$.metadata.name"
        comparisonType: ExactMatch
        defaultProps:
          - fieldA: "$.spec.volumes[].name"
            fieldB: ""
            default: "config-volume"
`,
			expectedRules: []RelationshipRule{
				{
					KindA:        "pods",
					KindB:        "configmaps",
					Relationship: "POD_USE_CONFIGMAP",
					MatchCriteria: []MatchCriterion{
						{
							FieldA:         "$.spec.volumes[].configMap.name",
							FieldB:         "$.metadata.name",
							ComparisonType: ExactMatch,
							DefaultProps: []DefaultProp{
								{
									FieldA:  "$.spec.volumes[].name",
									FieldB:  "",
									Default: "config-volume",
								},
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "Missing required fields",
			yamlContent: `
relationships:
  - kindA: pods
    relationship: INVALID_RULE
    matchCriteria:
      - fieldA: "$.spec.volumes[].configMap.name"
        comparisonType: ExactMatch
`,
			expectedError: true,
			errorContains: "kindB and relationship are required",
		},
		{
			name: "Invalid comparison type",
			yamlContent: `
relationships:
  - kindA: pods
    kindB: configmaps
    relationship: POD_USE_CONFIGMAP
    matchCriteria:
      - fieldA: "$.spec.volumes[].configMap.name"
        fieldB: "$.metadata.name"
        comparisonType: InvalidType
`,
			expectedError: true,
			errorContains: "must be ExactMatch, ContainsAll, or StringContains",
		},
		{
			name: "wildcard match with no known resource kinds",
			yamlContent: `
relationships:
  - kindA: '*'
    kindB: application
    relationship: ARGO_APP_CHILDREN
    matchCriteria:
      - fieldA: "$.spec.volumes[].configMap.name"
        fieldB: "$.metadata.name"
        comparisonType: StringContains
`,
			expectedError:      false,
			errorContains:      "",
			expectedRules:      []RelationshipRule{},
			knownResourceKinds: nil,
		},
		{
			name: "wildcard match with pod as a known resource kind",
			yamlContent: `
relationships:
  - kindA: '*'
    kindB: application
    relationship: ARGO_APP_CHILDREN
    matchCriteria:
      - fieldA: '$.metadata.annotation.argoproj\.io/tracking-id'
        fieldB: "$.metadata.name"
        comparisonType: StringContains
`,
			expectedError: false,
			errorContains: "",
			expectedRules: []RelationshipRule{
				{
					KindA:        "pod",
					KindB:        "application",
					Relationship: "ARGO_APP_CHILDREN_POD",
					MatchCriteria: []MatchCriterion{
						{
							FieldA:         "$.metadata.annotation.argoproj\\.io/tracking-id",
							FieldB:         "$.metadata.name",
							ComparisonType: StringContains,
						},
					},
				},
			},
			knownResourceKinds: []string{"Pod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test YAML to file
			yamlPath := filepath.Join(cyphernetesDir, "relationships.yaml")
			if err := os.WriteFile(yamlPath, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatal(err)
			}

			// Reset global relationshipRules
			originalRules := relationshipRules
			relationshipRules = []RelationshipRule{}
			defer func() {
				relationshipRules = originalRules
			}()

			// Test loading custom relationships
			_, err := loadCustomRelationships(tt.knownResourceKinds)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q but got %q", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check if expected rules were loaded
			for _, expectedRule := range tt.expectedRules {
				found := false
				for _, actualRule := range relationshipRules {
					if reflect.DeepEqual(expectedRule, actualRule) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected rule not found: %+v", expectedRule)
				}
			}
		})
	}
}

func TestStringContainsComparison(t *testing.T) {
	tests := []struct {
		name        string
		resourceA   interface{}
		resourceB   interface{}
		criterion   MatchCriterion
		expectMatch bool
	}{
		{
			name: "String contains match",
			resourceA: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-deployment",
				},
			},
			resourceB: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
			},
			criterion: MatchCriterion{
				FieldA:         "$.metadata.name",
				FieldB:         "$.metadata.name",
				ComparisonType: StringContains,
			},
			expectMatch: true,
		},
		{
			name: "String contains no match",
			resourceA: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "production-deployment",
				},
			},
			resourceB: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
			},
			criterion: MatchCriterion{
				FieldA:         "$.metadata.name",
				FieldB:         "$.metadata.name",
				ComparisonType: StringContains,
			},
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchByCriterion(tt.resourceA, tt.resourceB, tt.criterion)
			if result != tt.expectMatch {
				t.Errorf("matchByCriterion() with criterion %+v = %v, want %v",
					tt.criterion, result, tt.expectMatch)
			}
		})
	}
}
