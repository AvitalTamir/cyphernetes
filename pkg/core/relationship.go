package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"gopkg.in/yaml.v2"
)

var (
	relationshipsMutex sync.RWMutex
	relationships      = make(map[string][]string)
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

func InitializeRelationships(resourceSpecs map[string][]string, provider provider.Provider) {
	logDebug("Starting relationship initialization with", len(resourceSpecs), "resource specs")
	fmt.Print("🧠 Initializing relationships")
	relationshipCount := 0
	totalKinds := len(resourceSpecs)
	processed := 0
	lastProgress := 0

	// Regular expression to match fields ending with 'Name', or 'Ref'
	nameOrKeyRefFieldRegex := regexp.MustCompile(`(\w+)(Name|KeyRef)`)
	refFieldRegex := regexp.MustCompile(`(\w+)(Ref)`)

	for kindA, fields := range resourceSpecs {
		kindANameSingular := extractKindFromSchemaName(kindA)
		if kindANameSingular == "" {
			processed++
			continue
		}

		// Update progress bar
		progress := (processed * 100) / totalKinds
		if progress > lastProgress {
			fmt.Printf("\033[K\r🧠 Initializing relationships [%-25s] %d%%",
				strings.Repeat("=", progress/4),
				progress)
			lastProgress = progress
		}

		for _, fieldPath := range fields {
			parts := strings.Split(fieldPath, ".")
			fieldName := parts[len(parts)-1]

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

			// Use FindGVR to get the GVR for the related kind
			if gvr, err := provider.FindGVR(relatedKindSingular); err == nil {
				relatedKind := gvr.Resource

				// Same conversion for kindA
				if gvrA, err := provider.FindGVR(kindANameSingular); err == nil {
					kindAName := gvrA.Resource

					// Important: Keep the array notation in the field path
					fullFieldPath := fieldPath
					if relSpecType == "Ref" || relSpecType == "KeyRef" {
						fullFieldPath = fieldPath + ".name"
					}
					logDebug("Using field path:", fullFieldPath)

					relType := RelationshipType(fmt.Sprintf("%s_INSPEC_%s",
						strings.ToUpper(relatedKindSingular),
						strings.ToUpper(kindANameSingular)))

					kindA := strings.ToLower(kindAName)
					kindB := strings.ToLower(relatedKind)

					// Keep the array notation in the JsonPath
					fieldA := "$." + fullFieldPath
					fieldB := "$.metadata.name"

					logDebug("Creating relationship rule:", kindA, "->", kindB, "with fields:", fieldA, "->", fieldB)
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
						logDebug("Adding criterion to existing rule for:", kindA, "->", kindB)
						relationshipRules[existingRuleIndex].MatchCriteria = append(
							relationshipRules[existingRuleIndex].MatchCriteria,
							criterion,
						)
					} else {
						logDebug("Creating new relationship rule for:", kindA, "->", kindB)
						// Create new rule
						rule := RelationshipRule{
							KindA:         kindA,
							KindB:         kindB,
							Relationship:  relType,
							MatchCriteria: []MatchCriterion{criterion},
						}
						relationshipRules = append(relationshipRules, rule)
						relationshipCount++
					}
				}
			}
		}

		processed++
	}

	customRelationshipsCount, err := loadCustomRelationships()
	if err != nil {
		fmt.Println("\nError loading custom relationships:", err)
	}

	suffix := ""
	if customRelationshipsCount > 0 {
		suffix = fmt.Sprintf(" and %d custom", customRelationshipsCount)
	}

	logDebug("Relationship initialization complete. Found", relationshipCount, "internal relationships and", customRelationshipsCount, "custom relationships")
	fmt.Printf("\033[K\r ✔️ Initializing relationships (%d internal%s processed)\n", relationshipCount, suffix)
}

func loadCustomRelationships() (int, error) {
	counter := 0
	// Get user's home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, fmt.Errorf("error getting home directory: %v", err)
	}

	// Check if .cyphernetes/relationships.yaml exists
	relationshipsPath := filepath.Join(home, ".cyphernetes", "relationships.yaml")
	if _, err := os.Stat(relationshipsPath); os.IsNotExist(err) {
		return 0, nil
	}

	// Read and parse relationships.yaml
	data, err := os.ReadFile(relationshipsPath)
	if err != nil {
		return 0, fmt.Errorf("error reading relationships file: %v", err)
	}

	type CustomRelationships struct {
		Relationships []RelationshipRule `yaml:"relationships"`
	}

	var customRels CustomRelationships
	if err := yaml.Unmarshal(data, &customRels); err != nil {
		return 0, fmt.Errorf("error parsing relationships file: %v", err)
	}

	// Validate and add custom relationships
	for _, rule := range customRels.Relationships {
		// Validate required fields
		if rule.KindA == "" || rule.KindB == "" {
			return 0, fmt.Errorf("invalid relationship rule: kindA, kindB and relationship are required: %+v", rule)
		}
		if len(rule.MatchCriteria) == 0 {
			return 0, fmt.Errorf("invalid relationship rule: at least one match criterion is required: %+v", rule)
		}

		// Validate each criterion
		for _, criterion := range rule.MatchCriteria {
			if criterion.FieldA == "" || criterion.FieldB == "" {
				return 0, fmt.Errorf("invalid match criterion: fieldA and fieldB are required: %+v", criterion)
			}
			if criterion.ComparisonType != ExactMatch &&
				criterion.ComparisonType != ContainsAll &&
				criterion.ComparisonType != StringContains {
				return 0, fmt.Errorf("invalid comparison type: must be ExactMatch, ContainsAll, or StringContains: %v", criterion.ComparisonType)
			}
		}

		// Add to global relationships
		counter++
		AddRelationshipRule(rule)
	}

	return counter, nil
}

func AddRelationship(resourceA, resourceB interface{}, relationshipType string) {
	relationshipsMutex.Lock()
	defer relationshipsMutex.Unlock()

	// Add relationship logic...
}

func GetRelationships() map[string][]string {
	relationshipsMutex.RLock()
	defer relationshipsMutex.RUnlock()

	return relationships
}
