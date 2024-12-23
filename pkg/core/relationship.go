package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

func AddRelationshipRule(rule RelationshipRule) {
	relationshipRules = append(relationshipRules, rule)
}

func GetRelationshipRules() []RelationshipRule {
	return relationshipRules
}

func findRuleByRelationshipType(relType RelationshipType) (RelationshipRule, error) {
	for _, rule := range relationshipRules {
		if rule.Relationship == relType {
			return rule, nil
		}
	}
	return RelationshipRule{}, fmt.Errorf("no rule found for relationship type: %s", relType)
}

func applyRelationshipRule(resourcesA, resourcesB []map[string]interface{}, rule RelationshipRule, direction Direction) map[string]interface{} {
	var matchedResourcesA []map[string]interface{}
	var matchedResourcesB []map[string]interface{}

	for _, resourceA := range resourcesA {
		for _, resourceB := range resourcesB {
			if matchByCriteria(resourceA, resourceB, rule.MatchCriteria) {
				if direction == Left {
					if !containsResource(matchedResourcesA, resourceA) {
						matchedResourcesA = append(matchedResourcesA, resourceA)
					}
					if !containsResource(matchedResourcesB, resourceB) {
						matchedResourcesB = append(matchedResourcesB, resourceB)
					}
				} else if direction == Right {
					if !containsResource(matchedResourcesA, resourceB) {
						matchedResourcesA = append(matchedResourcesA, resourceB)
					}
					if !containsResource(matchedResourcesB, resourceA) {
						matchedResourcesB = append(matchedResourcesB, resourceA)
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"right": matchedResourcesA,
		"left":  matchedResourcesB,
	}
}

func containsResource(resources []map[string]interface{}, resource map[string]interface{}) bool {
	metadata1, ok1 := resource["metadata"].(map[string]interface{})
	if !ok1 {
		return false
	}
	name1, ok1 := metadata1["name"].(string)
	if !ok1 {
		return false
	}

	for _, res := range resources {
		metadata2, ok2 := res["metadata"].(map[string]interface{})
		if !ok2 {
			continue
		}
		name2, ok2 := metadata2["name"].(string)
		if !ok2 {
			continue
		}
		if name1 == name2 {
			return true
		}
	}
	return false
}

func InitializeRelationships(specs map[string][]string) {
	// Process OpenAPI specs to discover relationships
	for kindA, fields := range specs {
		for _, field := range fields {
			if strings.Contains(field, "metadata.name") {
				// This could indicate a reference
				parts := strings.Split(field, ".")
				if len(parts) > 2 {
					kindB := parts[0] // The referenced kind
					AddRelationshipRule(RelationshipRule{
						KindA:        kindA,
						KindB:        kindB,
						Relationship: RelationshipType(fmt.Sprintf("%s_REFERENCES_%s", kindA, kindB)),
						MatchCriteria: []MatchCriterion{
							{
								FieldA:         fmt.Sprintf("$.%s", field),
								FieldB:         "$.metadata.name",
								ComparisonType: ExactMatch,
							},
						},
					})
				}
			}
		}
	}
}

func loadCustomRelationships() error {
	// Get user's home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting home directory: %v", err)
	}

	// Check if .cyphernetes/relationships.yaml exists
	relationshipsPath := filepath.Join(home, ".cyphernetes", "relationships.yaml")
	if _, err := os.Stat(relationshipsPath); os.IsNotExist(err) {
		return nil
	}

	// Read and parse relationships.yaml
	data, err := os.ReadFile(relationshipsPath)
	if err != nil {
		return fmt.Errorf("error reading relationships file: %v", err)
	}

	type CustomRelationships struct {
		Relationships []RelationshipRule `yaml:"relationships"`
	}

	var customRels CustomRelationships
	if err := yaml.Unmarshal(data, &customRels); err != nil {
		return fmt.Errorf("error parsing relationships file: %v", err)
	}

	// Validate and add custom relationships
	for _, rule := range customRels.Relationships {
		// Validate required fields
		if rule.KindA == "" || rule.KindB == "" {
			return fmt.Errorf("invalid relationship rule: kindA, kindB and relationship are required: %+v", rule)
		}
		if len(rule.MatchCriteria) == 0 {
			return fmt.Errorf("invalid relationship rule: at least one match criterion is required: %+v", rule)
		}

		// Validate each criterion
		for _, criterion := range rule.MatchCriteria {
			if criterion.FieldA == "" || criterion.FieldB == "" {
				return fmt.Errorf("invalid match criterion: fieldA and fieldB are required: %+v", criterion)
			}
			if criterion.ComparisonType != ExactMatch &&
				criterion.ComparisonType != ContainsAll &&
				criterion.ComparisonType != StringContains {
				return fmt.Errorf("invalid comparison type: must be ExactMatch, ContainsAll, or StringContains: %v", criterion.ComparisonType)
			}
		}

		// Add to global relationships
		AddRelationshipRule(rule)
	}

	return nil
}
