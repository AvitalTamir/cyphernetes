package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

		// Get GVR for kindA, handling ambiguity
		gvrA, err := tryResolveGVR(provider, kindANameSingular)
		if err != nil {
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

			// Get GVR for related kind, handling ambiguity
			gvr, err := tryResolveGVR(provider, relatedKindSingular)
			if err != nil {
				continue
			}

			// Important: Keep the array notation in the field path
			fullFieldPath := fieldPath
			if relSpecType == "Ref" || relSpecType == "KeyRef" {
				fullFieldPath = fieldPath + ".name"
			}
			logDebug("Using field path:", fullFieldPath)

			relType := RelationshipType(fmt.Sprintf("%s_INSPEC_%s",
				strings.ToUpper(relatedKindSingular),
				strings.ToUpper(kindANameSingular)))

			kindA := strings.ToLower(gvrA.Resource)
			kindB := strings.ToLower(gvr.Resource)

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

// Helper function to try resolving GVR with core prefix if ambiguous
func tryResolveGVR(provider provider.Provider, kind string) (schema.GroupVersionResource, error) {
	gvr, err := provider.FindGVR(kind)
	if err != nil {
		if strings.Contains(err.Error(), "ambiguous") {
			// Try with core. prefix
			gvr, err = provider.FindGVR("core." + kind)
			if err == nil {
				return gvr, nil
			}
			// If that fails, try extracting the core option from the ambiguous error
			options := strings.Split(err.Error(), "\n")
			for _, option := range options {
				if strings.HasPrefix(option, "core.") {
					return provider.FindGVR(option)
				}
			}
		}
		return schema.GroupVersionResource{}, err
	}
	return gvr, nil
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

// FindPotentialKinds returns all possible target kinds that could have a relationship with the given source kind
func FindPotentialKinds(sourceKind string) []string {
	sourceKind = strings.ToLower(sourceKind)
	potentialKinds := make(map[string]bool)
	debugLog("FindPotentialKinds: looking for relationships for sourceKind=%s", sourceKind)

	// Look through all relationship rules
	rules := GetRelationshipRules()
	debugLog("FindPotentialKinds: found %d relationship rules", len(rules))

	for _, rule := range rules {
		debugLog("FindPotentialKinds: checking rule KindA=%s, KindB=%s, Relationship=%s", rule.KindA, rule.KindB, rule.Relationship)

		// If sourceKind is KindB, we want KindA (what can connect to sourceKind)
		if strings.ToLower(rule.KindB) == sourceKind || strings.ToLower(rule.KindB) == sourceKind+"s" {
			debugLog("FindPotentialKinds: matched KindB, adding KindA=%s", rule.KindA)
			potentialKinds[rule.KindA] = true
		}
		// If sourceKind is KindA, we want KindB (what sourceKind can connect to)
		if strings.ToLower(rule.KindA) == sourceKind || strings.ToLower(rule.KindA) == sourceKind+"s" {
			debugLog("FindPotentialKinds: matched KindA, adding KindB=%s", rule.KindB)
			potentialKinds[rule.KindB] = true
		}
	}

	// Convert map to slice
	var result []string
	for kind := range potentialKinds {
		result = append(result, kind)
	}
	sort.Strings(result) // Sort for consistent results
	debugLog("FindPotentialKinds: final result for %s = %v", sourceKind, result)
	return result
}

// FindPotentialKindsIntersection returns the intersection of possible kinds from multiple relationships
func FindPotentialKindsIntersection(relationships []*Relationship) []string {
	if len(relationships) == 0 {
		debugLog("FindPotentialKindsIntersection: no relationships provided")
		return []string{}
	}

	// Check if there are any unknown kinds that need resolution
	hasUnknownKind := false
	for _, rel := range relationships {
		if rel.LeftNode.ResourceProperties.Kind == "" || rel.RightNode.ResourceProperties.Kind == "" {
			hasUnknownKind = true
			break
		}
	}

	// If all kinds are known, return empty slice
	if !hasUnknownKind {
		debugLog("FindPotentialKindsIntersection: all kinds are known")
		return []string{}
	}

	// Find all known kinds in the relationships
	knownKinds := make(map[string]bool)
	for _, rel := range relationships {
		if rel.LeftNode.ResourceProperties.Kind != "" {
			knownKinds[strings.ToLower(rel.LeftNode.ResourceProperties.Kind)] = true
		}
		if rel.RightNode.ResourceProperties.Kind != "" {
			knownKinds[strings.ToLower(rel.RightNode.ResourceProperties.Kind)] = true
		}
	}
	debugLog("FindPotentialKindsIntersection: found known kinds=%v", knownKinds)

	// If no known kinds, return empty
	if len(knownKinds) == 0 {
		debugLog("FindPotentialKindsIntersection: no known kinds found")
		return []string{}
	}

	// Initialize result with potential kinds from first known kind
	var firstKnownKind string
	for kind := range knownKinds {
		firstKnownKind = kind
		break
	}

	result := make(map[string]bool)
	for _, kind := range FindPotentialKinds(firstKnownKind) {
		result[kind] = true
	}
	debugLog("FindPotentialKindsIntersection: initial potential kinds from %s=%v", firstKnownKind, result)

	// For each additional known kind, intersect with its potential kinds
	for kind := range knownKinds {
		if kind == firstKnownKind {
			continue
		}

		potentialKinds := FindPotentialKinds(kind)
		debugLog("FindPotentialKindsIntersection: potential kinds for %s=%v", kind, potentialKinds)

		newResult := make(map[string]bool)
		// Keep only kinds that exist in both sets
		for _, potentialKind := range potentialKinds {
			if result[potentialKind] {
				debugLog("FindPotentialKindsIntersection: keeping common kind %s", potentialKind)
				newResult[potentialKind] = true
			}
		}
		result = newResult
	}

	// Convert map back to slice
	var kinds []string
	for kind := range result {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds) // Sort for consistent results
	debugLog("FindPotentialKindsIntersection: final result=%v", kinds)
	return kinds
}
