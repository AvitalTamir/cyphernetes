package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
			matched := false
			for _, criterion := range rule.MatchCriteria {
				if matchByCriterion(resourceA, resourceB, criterion) {
					matched = true
					break
				}
			}

			if matched {
				if !containsResource(matchedResourcesA, resourceA) {
					matchedResourcesA = append(matchedResourcesA, resourceA)
				}
				if !containsResource(matchedResourcesB, resourceB) {
					matchedResourcesB = append(matchedResourcesB, resourceB)
				}
			}
		}
	}

	if direction == Left {
		return map[string]interface{}{
			"right": matchedResourcesA,
			"left":  matchedResourcesB,
		}
	} else {
		return map[string]interface{}{
			"right": matchedResourcesB,
			"left":  matchedResourcesA,
		}
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

func InitializeRelationships(resourceSpecs map[string][]string) {
	// Map to hold available kinds for quick look-up
	availableKinds := make(map[string]bool)
	for schemaName := range resourceSpecs {
		kind := extractKindFromSchemaName(schemaName)
		if kind != "" {
			// fmt.Println("Adding available kind:", kind)
			availableKinds[strings.ToLower(kind)] = true
		}
	}

	// Regular expression to match fields ending with 'Name', or 'Ref'
	nameOrKeyRefFieldRegex := regexp.MustCompile(`(\w+)(Name|KeyRef)`)
	refFieldRegex := regexp.MustCompile(`(\w+)(Ref)`)

	for kindA, fields := range resourceSpecs {
		kindANameSingular := extractKindFromSchemaName(kindA)
		if kindANameSingular == "" {
			continue
		}

		for _, fieldPath := range fields {
			parts := strings.Split(fieldPath, ".")
			fieldName := parts[len(parts)-1]

			// Remove the special handling for configmap and instead improve the general logic
			relatedKindSingular := ""
			relSpecType := ""

			if nameOrKeyRefFieldRegex.MatchString(fieldName) {
				relatedKindSingular = nameOrKeyRefFieldRegex.ReplaceAllString(fieldName, "$1")
				relSpecType = nameOrKeyRefFieldRegex.ReplaceAllString(fieldName, "$2")
			} else if refFieldRegex.MatchString(fieldName) {
				relatedKindSingular = refFieldRegex.ReplaceAllString(fieldName, "$1")
				relSpecType = refFieldRegex.ReplaceAllString(fieldName, "$2")
			} else {
				continue
			}

			// convert relatedKind to lower case plural using GVR cache
			if gvr, ok := GvrCache[strings.ToLower(relatedKindSingular)]; ok {
				relatedKind := gvr.Resource

				// same conversion for kindA
				if gvrA, ok := GvrCache[strings.ToLower(kindANameSingular)]; ok {
					kindAName := gvrA.Resource

					// Important: Keep the array notation in the field path
					fullFieldPath := fieldPath
					if relSpecType == "Ref" || relSpecType == "KeyRef" {
						fullFieldPath = fieldPath + ".name"
					}

					// Check if relatedKind exists in availableKinds
					if _, exists := availableKinds[strings.ToLower(relatedKindSingular)]; exists {
						relType := RelationshipType(fmt.Sprintf("%s_INSPEC_%s",
							strings.ToUpper(relatedKindSingular),
							strings.ToUpper(kindANameSingular)))

						kindA := strings.ToLower(kindAName)
						kindB := strings.ToLower(relatedKind)

						// Keep the array notation in the JsonPath
						fieldA := "$." + fullFieldPath
						fieldB := "$.metadata.name"

						criterion := MatchCriterion{
							FieldA:         fieldA,
							FieldB:         fieldB,
							ComparisonType: ExactMatch,
						}

						// Check for existing rule and add/create as before
						existingRuleIndex := -1
						for i, r := range relationshipRules {
							if r.KindA == kindA && r.KindB == kindB && r.Relationship == relType {
								existingRuleIndex = i
								break
							}
						}

						if existingRuleIndex >= 0 {
							relationshipRules[existingRuleIndex].MatchCriteria = append(
								relationshipRules[existingRuleIndex].MatchCriteria,
								criterion,
							)
						} else {
							// Create new rule
							rule := RelationshipRule{
								KindA:         kindA,
								KindB:         kindB,
								Relationship:  relType,
								MatchCriteria: []MatchCriterion{criterion},
							}
							relationshipRules = append(relationshipRules, rule)
						}
					}
				}
			}
		}
	}

	err := loadCustomRelationships()
	if err != nil {
		fmt.Println("Error loading custom relationships:", err)
	}
}

func loadCustomRelationships() error {
	counter := 0
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
		counter++
		AddRelationshipRule(rule)
	}

	suffix := ""
	if counter > 0 {
		suffix = "s"
	}
	fmt.Printf("💡 added %d custom relationship%s\n", counter, suffix)

	return nil
}
