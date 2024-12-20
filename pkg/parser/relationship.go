package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/AvitalTamir/jsonpath"
)

type CustomRelationships struct {
	Relationships []RelationshipRule `yaml:"relationships"`
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
		logDebug("No relationships.yaml file found at", relationshipsPath)
		return nil
	}

	// Read and parse relationships.yaml
	data, err := os.ReadFile(relationshipsPath)
	if err != nil {
		return fmt.Errorf("error reading relationships file: %v", err)
	}

	logDebug("Read relationships file contents:", string(data))

	var customRels CustomRelationships
	if err := yaml.Unmarshal(data, &customRels); err != nil {
		return fmt.Errorf("error parsing relationships file: %v", err)
	}

	logDebug("Parsed relationships:", fmt.Sprintf("%+v", customRels))

	customRelationshipCount := 0
	// Validate and add custom relationships
	for _, rule := range customRels.Relationships {
		logDebug("Processing rule:", fmt.Sprintf("%+v", rule))

		// Validate required fields
		if rule.KindA == "" || rule.KindB == "" || string(rule.Relationship) == "" {
			return fmt.Errorf("invalid relationship rule: kindA, kindB and relationship are required: %+v", rule)
		}
		if len(rule.MatchCriteria) == 0 {
			return fmt.Errorf("invalid relationship rule: at least one match criterion is required: %+v", rule)
		}

		// Validate each criterion
		for _, criterion := range rule.MatchCriteria {
			logDebug("Processing criterion:", fmt.Sprintf("%+v", criterion))

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
		relationshipRules = append(relationshipRules, rule)
		customRelationshipCount++
		logDebug("Added rule successfully:", fmt.Sprintf("%+v", rule))
	}
	pluralExt := "s"
	if customRelationshipCount == 1 {
		pluralExt = ""
	}
	fmt.Println("💡 added", customRelationshipCount, "custom relationship", pluralExt)
	return nil
}

func initializeRelationships() {
	// Map to hold available kinds for quick look-up
	availableKinds := make(map[string]bool)
	for schemaName := range ResourceSpecs {
		kind := extractKindFromSchemaName(schemaName)
		if kind != "" {
			availableKinds[kind] = true
		}
	}

	// Regular expression to match fields ending with 'Name', or 'Ref'
	nameOrKeyRefFieldRegex := regexp.MustCompile(`(\w+)(Name|KeyRef)`)
	refFieldRegex := regexp.MustCompile(`(\w+)(Ref)`)

	// Load custom relationships
	if err := loadCustomRelationships(); err != nil {
		logDebug("Error loading custom relationships:", err)
	}

	for kindA, fields := range ResourceSpecs {
		kindANameSingular := extractKindFromSchemaName(kindA)

		// Skip if kindAName is empty
		if kindANameSingular == "" {
			continue
		}

		for _, fieldPath := range fields {
			parts := strings.Split(fieldPath, ".")
			fieldName := parts[len(parts)-1]

			relatedKindSingular := ""
			relSpecType := ""

			if nameOrKeyRefFieldRegex.MatchString(fieldName) {
				// Extract potential KindB
				relatedKindSingular = nameOrKeyRefFieldRegex.ReplaceAllString(fieldName, "$1")
				relSpecType = nameOrKeyRefFieldRegex.ReplaceAllString(fieldName, "$2")
			} else if refFieldRegex.MatchString(fieldName) {
				// Extract potential KindB
				relatedKindSingular = refFieldRegex.ReplaceAllString(fieldName, "$1")
				relSpecType = refFieldRegex.ReplaceAllString(fieldName, "$2")
			} else {
				continue
			}

			// convert relatedKind to lower case plural - find the correct plural form using FindGVR
			gvr, err := FindGVR(executorInstance.Clientset, relatedKindSingular)
			if err != nil {
				continue
			}
			relatedKind := gvr.Resource

			// same converstion for kindA
			gvr, err = FindGVR(executorInstance.Clientset, kindANameSingular)
			if err != nil {
				continue
			}
			kindAName := gvr.Resource

			if relSpecType == "Ref" || relSpecType == "KeyRef" {
				fieldPath = fieldPath + ".name"
			}
			// Check if relatedKind exists in availableKinds
			if _, exists := availableKinds[strings.ToLower(relatedKindSingular)]; exists {
				// Create a new relationship rule if one doesn't already exist between these two kinds.
				// If it does exist, append the new criterion to the existing rule's match criteria.
				relType := RelationshipType(fmt.Sprintf("%s%s_INSPEC_%s", strings.ToUpper(relatedKindSingular), strings.ToUpper(relSpecType), strings.ToUpper(kindANameSingular)))
				rule, err := findRuleByKinds(strings.ToLower(kindAName), strings.ToLower(relatedKind))
				kindA = strings.ToLower(kindAName)
				kindB := strings.ToLower(relatedKind)
				fieldA := "$." + fieldPath
				fieldB := "$.metadata.name"
				if err == nil {
					if rule.KindA == kindB && rule.KindB == kindA {
						fieldA = "$.metadata.name"
						fieldB = "$." + fieldPath
					}
					rule.MatchCriteria = append(rule.MatchCriteria, MatchCriterion{
						FieldA:         fieldA,
						FieldB:         fieldB,
						ComparisonType: ExactMatch,
					})
				} else {
					rule = RelationshipRule{
						KindA:        kindA,
						KindB:        kindB,
						Relationship: relType,
						MatchCriteria: []MatchCriterion{
							{
								FieldA:         fieldA,
								FieldB:         fieldB,
								ComparisonType: ExactMatch,
							},
						},
					}
					// Append the new rule to existing relationshipRules
					relationshipRules = append(relationshipRules, rule)
				}
			}
		}
	}
}

func findRuleByRelationshipType(relationshipType RelationshipType) (RelationshipRule, error) {
	for _, rule := range relationshipRules {
		if rule.Relationship == relationshipType {
			return rule, nil
		}
	}
	return RelationshipRule{}, fmt.Errorf("rule not found for relationship type: %s", relationshipType)
}

func findRuleByKinds(kindA, kindB string) (RelationshipRule, error) {
	for _, rule := range relationshipRules {
		if (rule.KindA == kindA && rule.KindB == kindB) || (rule.KindA == kindB && rule.KindB == kindA) {
			return rule, nil
		}
	}
	return RelationshipRule{}, fmt.Errorf("rule not found for kinds: %s and %s", kindA, kindB)
}

func matchByCriteria(resourceA, resourceB interface{}, criteria []MatchCriterion) bool {
	for _, criterion := range criteria {
		switch criterion.ComparisonType {
		case ContainsAll:
			l, err := jsonpath.JsonPathLookup(resourceA, strings.ReplaceAll(criterion.FieldA, "[]", ""))
			if err != nil {
				logDebug("Error extracting fieldA: ", err)
				return false
			}
			labels, ok := l.(map[string]interface{})
			if !ok {
				logDebug("No labels found for resource: ", resourceA)
				return false
			}

			s, err := jsonpath.JsonPathLookup(resourceB, strings.ReplaceAll(criterion.FieldB, "[]", ""))
			if err != nil {
				logDebug("Error extracting fieldB: ", err)
				return false
			}
			selector, ok := s.(map[string]interface{})
			if !ok {
				logDebug("No resources found for selector: ", selector)
				return false
			}

			if !matchContainsAll(labels, selector) {
				return false
			}
		case ExactMatch:
			// Logic for exact field matching

			// Extract the fields
			fieldsA, err := jsonpath.JsonPathLookup(resourceA, strings.ReplaceAll(criterion.FieldA, "[]", ""))
			if err != nil {
				logDebug("Error extracting fieldA: ", err)
				return false
			}
			fieldsB, err := jsonpath.JsonPathLookup(resourceB, strings.ReplaceAll(criterion.FieldB, "[]", ""))
			if err != nil {
				logDebug("Error extracting fieldB: ", err)
				return false
			}
			if !matchFields(fieldsA, fieldsB) {
				return false
			}
		case StringContains:
			// Extract the fields
			fieldA, err := jsonpath.JsonPathLookup(resourceA, strings.ReplaceAll(criterion.FieldA, "[]", ""))
			if err != nil {
				logDebug("Error extracting fieldA: ", err)
				return false
			}
			fieldB, err := jsonpath.JsonPathLookup(resourceB, strings.ReplaceAll(criterion.FieldB, "[]", ""))
			if err != nil {
				logDebug("Error extracting fieldB: ", err)
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
	// if fieldA contains fieldB on some nested level, no matter how deep, return true
	// otherwise return false. make sure to recurse into all levels of the fieldA

	// if fieldA is a string, compare it to fieldB
	fieldAString, ok := fieldA.(string)
	if ok {
		return fieldAString == fieldB
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
		return fieldANumber == fieldB
	}

	// if fieldA is a bool, compare it to fieldB
	fieldABool, ok := fieldA.(bool)
	if ok {
		return fieldABool == fieldB
	}

	// if fieldA is nil, return false
	if fieldA == nil {
		return false
	}

	// if fieldA is anything else, return false
	return false
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

func applyRelationshipRule(resourcesA, resourcesB []map[string]interface{}, rule RelationshipRule, direction Direction) map[string]interface{} {
	var matchedResourcesA []map[string]interface{}
	var matchedResourcesB []map[string]interface{}

	for _, resourceA := range resourcesA {
		for _, resourceB := range resourcesB {
			if matchByCriteria(resourceA, resourceB, rule.MatchCriteria) {
				if direction == Left {
					// if resourceA doesn't already exist in matchedResourcesA, add it
					if !containsResource(matchedResourcesA, resourceA) {
						matchedResourcesA = append(matchedResourcesA, resourceA)
					}
					// if resourceB doesn't already exist in matchedResourcesB, add it
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

	// initialize matchedResources map
	matchedResources := make(map[string]interface{})

	// return the matched resources as a slice of maps that looks like this:
	// matchedresources["right"] = matchedResourcesA
	// matchedresources["left"] = matchedResourcesB
	matchedResources["right"] = matchedResourcesA
	matchedResources["left"] = matchedResourcesB

	return matchedResources
}

func containsResource(resources []map[string]interface{}, resource map[string]interface{}) bool {
	for _, res := range resources {
		if res["metadata"].(map[string]interface{})["name"] == resource["metadata"].(map[string]interface{})["name"] {
			return true
		}
	}
	return false
}
