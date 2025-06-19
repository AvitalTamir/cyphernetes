package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
	"github.com/gobwas/glob"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	relationshipsMutex  sync.RWMutex
	relationships       = make(map[string][]string)
	potentialKindsCache = make(map[string][]string) // kind -> potential kinds
	potentialKindsMutex sync.RWMutex
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

	ns1, ok1 := metadata1["namespace"].(string)
	if !ok1 {
		ns1 = ""
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
		ns2, _ := metadata2["namespace"].(string)
		if !ok2 {
			ns2 = ""
		}
		if name1 == name2 && ns1 == ns2 {
			return true
		}
	}
	return false
}

func InitializeRelationships(resourceSpecs map[string][]string, provider provider.Provider) {
	if CleanOutput {
		debugLog("Running relationship initialization in query mode (suppressing output)")
	} else {
		debugLog("Starting relationship initialization with %d resource specs", len(resourceSpecs))
	}

	// Only show progress bar if not in clean output mode
	if !CleanOutput {
		fmt.Print("🧠 Initializing relationships")
	}

	relationshipCount := 0
	totalKinds := len(resourceSpecs)
	processed := 0
	lastProgress := 0

	// Initialize potential kinds cache
	potentialKindsMutex.Lock()
	potentialKindsCache = make(map[string][]string)
	potentialKindsMutex.Unlock()

	// Regular expression to match fields ending with 'Name', or 'Ref'
	nameOrKeyRefFieldRegex := regexp.MustCompile(`(\w+)(Name|KeyRef)`)
	refFieldRegex := regexp.MustCompile(`(\w+)(Ref)`)

	// Map to collect potential kinds before sorting
	tempPotentialKinds := make(map[string]map[string]bool)

	// Cache for parent field GVR resolution within this function run
	parentGVRCache := make(map[string]schema.GroupVersionResource)

	// Pre-cache all GVRs for faster lookups
	gvrCache := make(map[string]schema.GroupVersionResource)

	// First, cache all the kinds we'll be resolving
	// Add existing rule kinds to ensure we have everything
	kindsToCache := make(map[string]bool)
	for kindA := range resourceSpecs {
		kindANameSingular := extractKindFromSchemaName(kindA)
		if kindANameSingular != "" {
			kindsToCache[strings.ToLower(kindANameSingular)] = true
		}
	}

	// Add existing rule kinds
	for _, rule := range relationshipRules {
		kindsToCache[rule.KindA] = true
		kindsToCache[rule.KindB] = true
	}

	// Populate the GVR cache
	for kind := range kindsToCache {
		gvr, err := tryResolveGVR(provider, kind)
		if err == nil {
			gvrCache[kind] = gvr
		}
	}

	debugLog("Pre-cached %d kind GVR resolutions", len(gvrCache))

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

		// Add the resolved GVR to the cache
		gvrCache[strings.ToLower(kindANameSingular)] = gvrA
		gvrCache[strings.ToLower(gvrA.Resource)] = gvrA

		// Update progress bar
		progress := (processed * 100) / totalKinds
		if progress > lastProgress {
			if !CleanOutput {
				fmt.Printf("\033[K\r🧠 Initializing relationships [%-25s] %d%%",
					strings.Repeat("=", progress/4),
					progress)
			}
			lastProgress = progress
		}

		for _, fieldPath := range fields {
			parts := strings.Split(fieldPath, ".")
			fieldName := parts[len(parts)-1]
			processedThisField := false

			// New logic: Check for "name" field with a GVR parent
			if fieldName == "name" && len(parts) > 1 {
				parentFieldName := parts[len(parts)-2]

				parentGVR, foundInCache := parentGVRCache[parentFieldName]
				if !foundInCache {
					var resolveErr error
					parentGVR, resolveErr = tryResolveGVR(provider, parentFieldName)
					if resolveErr == nil {
						parentGVRCache[parentFieldName] = parentGVR // Cache success
						// Add to the main GVR cache
						gvrCache[strings.ToLower(parentFieldName)] = parentGVR
						gvrCache[strings.ToLower(parentGVR.Resource)] = parentGVR
					} else {
						parentGVRCache[parentFieldName] = schema.GroupVersionResource{} // Cache failure
					}
				}

				if parentGVR != (schema.GroupVersionResource{}) {
					relatedKindSingularFromParentGVR := parentGVR.Resource

					ruleKindA := strings.ToLower(gvrA.Resource)
					ruleKindB := strings.ToLower(relatedKindSingularFromParentGVR)

					// First, check if a relationship rule already exists between these kinds
					existingRuleIndex, found := findExistingRelationshipRule(ruleKindA, ruleKindB, gvrCache)

					if found {
						// Use the existing relationship type for consistency
						existingRelType := relationshipRules[existingRuleIndex].Relationship
						debugLog("Found existing relationship rule %s for %s -> %s", existingRelType, ruleKindA, ruleKindB)

						// Create the new criterion
						fieldA := "$." + fieldPath  // Path to the "name" field itself
						fieldB := "$.metadata.name" // Standard target

						criterion := MatchCriterion{
							FieldA:         fieldA,
							FieldB:         fieldB,
							ComparisonType: ExactMatch,
						}

						// Check for duplicate criterion
						alreadyExists := false
						for _, existingCrit := range relationshipRules[existingRuleIndex].MatchCriteria {
							if existingCrit.FieldA == criterion.FieldA &&
								existingCrit.FieldB == criterion.FieldB &&
								existingCrit.ComparisonType == criterion.ComparisonType {
								alreadyExists = true
								break
							}
						}

						if !alreadyExists {
							// Add new criterion to existing rule
							debugLog("Adding criterion to existing rule %s for: %s -> %s", existingRelType, ruleKindA, ruleKindB)
							relationshipRules[existingRuleIndex].MatchCriteria = append(
								relationshipRules[existingRuleIndex].MatchCriteria,
								criterion,
							)
						}
					} else {
						// Create a new relationship using standard INSPEC format
						relType := RelationshipType(fmt.Sprintf("%s_INSPEC_%s",
							strings.ToUpper(relatedKindSingularFromParentGVR),
							strings.ToUpper(kindANameSingular)))

						// Create potential kinds entries
						var kindAFull string
						if gvrA.Group != "" {
							kindAFull = fmt.Sprintf("%s.%s", gvrA.Resource, gvrA.Group)
						} else {
							kindAFull = "core." + ruleKindA
						}

						var kindBFullName string
						if parentGVR.Group != "" {
							kindBFullName = fmt.Sprintf("%s.%s", parentGVR.Resource, parentGVR.Group)
						} else {
							kindBFullName = "core." + ruleKindB
						}

						if tempPotentialKinds[kindAFull] == nil {
							tempPotentialKinds[kindAFull] = make(map[string]bool)
						}
						if tempPotentialKinds[kindBFullName] == nil {
							tempPotentialKinds[kindBFullName] = make(map[string]bool)
						}
						tempPotentialKinds[kindAFull][kindBFullName] = true
						tempPotentialKinds[kindBFullName][kindAFull] = true

						fieldA := "$." + fieldPath  // Path to the "name" field itself
						fieldB := "$.metadata.name" // Standard target

						debugLog("Creating new relationship rule for: %s -> %s with fields: %s -> %s for relType: %s", ruleKindA, ruleKindB, fieldA, fieldB, relType)
						criterion := MatchCriterion{
							FieldA:         fieldA,
							FieldB:         fieldB,
							ComparisonType: ExactMatch,
						}

						// Create new rule
						debugLog("Creating new relationship rule for: %s -> %s", ruleKindA, ruleKindB)
						rule := RelationshipRule{
							KindA:         ruleKindA,
							KindB:         ruleKindB,
							Relationship:  relType,
							MatchCriteria: []MatchCriterion{criterion},
						}
						relationshipRules = append(relationshipRules, rule)
						relationshipCount++
					}

					processedThisField = true // Mark as processed by parent GVR logic
				}
			}

			// Original logic for fields ending with Name, KeyRef, or Ref
			if !processedThisField {
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

				fullFieldPath := fieldPath
				if relSpecType == "Ref" || relSpecType == "KeyRef" {
					fullFieldPath = fieldPath + ".name"
				}
				debugLog("Using field path: %s", fullFieldPath)

				relType := RelationshipType(fmt.Sprintf("%s_INSPEC_%s",
					strings.ToUpper(relatedKindSingular),
					strings.ToUpper(kindANameSingular)))

				kindA := strings.ToLower(gvrA.Resource)
				kindB := strings.ToLower(gvr.Resource)
				kindAFull := kindA
				kindBFull := kindB
				if gvrA.Group != "" {
					kindAFull = fmt.Sprintf("%s.%s", gvrA.Resource, gvrA.Group)
				} else {
					kindAFull = "core." + kindA
				}
				if gvr.Group != "" {
					kindBFull = fmt.Sprintf("%s.%s", gvr.Resource, gvr.Group)
				} else {
					kindBFull = "core." + kindB
				}

				// Update potential kinds cache for both directions
				if tempPotentialKinds[kindAFull] == nil {
					tempPotentialKinds[kindAFull] = make(map[string]bool)
				}
				if tempPotentialKinds[kindBFull] == nil {
					tempPotentialKinds[kindBFull] = make(map[string]bool)
				}
				tempPotentialKinds[kindAFull][kindBFull] = true
				tempPotentialKinds[kindBFull][kindAFull] = true

				// Keep the array notation in the JsonPath
				fieldA := "$." + fullFieldPath
				fieldB := "$.metadata.name"

				debugLog("Creating relationship rule: %s -> %s with fields: %s -> %s", kindA, kindB, fieldA, fieldB)
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
					debugLog("Adding criterion to existing rule for: %s -> %s", kindA, kindB)
					relationshipRules[existingRuleIndex].MatchCriteria = append(
						relationshipRules[existingRuleIndex].MatchCriteria,
						criterion,
					)
				} else {
					debugLog("Creating new relationship rule for: %s -> %s", kindA, kindB)
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

		processed++
	}

	// Convert temporary map to sorted slices and update cache
	potentialKindsMutex.Lock()
	for kind, potentialMap := range tempPotentialKinds {
		var sortedKinds []string
		for potentialKind := range potentialMap {
			sortedKinds = append(sortedKinds, potentialKind)
		}
		sort.Strings(sortedKinds)
		potentialKindsCache[kind] = sortedKinds
	}

	// Add hardcoded relationship rules to cache
	for _, rule := range relationshipRules {
		kindA := strings.ToLower(rule.KindA)
		kindB := strings.ToLower(rule.KindB)

		// resolve gvr for kindA and kindB
		gvrA, err := tryResolveGVR(provider, kindA)
		if err != nil {
			continue
		}
		gvrB, err := tryResolveGVR(provider, kindB)
		if err != nil {
			continue
		}

		var kindAFull, kindBFull string
		if gvrA.Group != "" {
			kindAFull = fmt.Sprintf("%s.%s", gvrA.Resource, gvrA.Group)
		} else {
			kindAFull = "core." + kindA
		}
		if gvrB.Group != "" {
			kindBFull = fmt.Sprintf("%s.%s", gvrB.Resource, gvrB.Group)
		} else {
			kindBFull = "core." + kindB
		}

		if potentialKindsCache[kindAFull] == nil {
			potentialKindsCache[kindAFull] = []string{}
		}
		if potentialKindsCache[kindBFull] == nil {
			potentialKindsCache[kindBFull] = []string{}
		}

		// Add B to A's potential kinds if not already present
		if !contains(potentialKindsCache[kindAFull], kindBFull) {
			potentialKindsCache[kindAFull] = append(potentialKindsCache[kindAFull], kindBFull)
			sort.Strings(potentialKindsCache[kindAFull])
		}

		// Add A to B's potential kinds if not already present
		if !contains(potentialKindsCache[kindBFull], kindAFull) {
			potentialKindsCache[kindBFull] = append(potentialKindsCache[kindBFull], kindAFull)
			sort.Strings(potentialKindsCache[kindBFull])
		}
	}
	potentialKindsMutex.Unlock()

	var knownResourceKinds []string
	serverProvider, ok := provider.(*apiserver.APIServerProvider)
	if ok {
		knownResourceKinds = serverProvider.GetKnownResourceKinds()
	}
	customRelationshipsCount, err := loadCustomRelationships(knownResourceKinds)
	if err != nil && !CleanOutput {
		fmt.Println("\nError loading custom relationships:", err)
	}

	suffix := ""
	if customRelationshipsCount > 0 {
		suffix = fmt.Sprintf(" and %d custom", customRelationshipsCount)
	}

	debugLog("Relationship initialization complete. Found %d internal relationships and %d custom relationships", relationshipCount, customRelationshipsCount)

	if !CleanOutput {
		fmt.Printf("\033[K\r ✔️ Initializing relationships (%d internal%s processed)\n", relationshipCount, suffix)
	}
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

func loadCustomRelationships(knownResourceKinds []string) (int, error) {
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

		// if there is any glob pattern in the rule's kind, evaluate the glob pattern and
		// find matching resource kinds that is available in the cluster.
		if strings.ContainsAny(rule.KindA, "*?[]{}") {
			globA, err := glob.Compile(rule.KindA)
			if err != nil {
				return 0, err
			}
			for _, gvrName := range knownResourceKinds {
				if !globA.Match(strings.ToLower(gvrName)) {
					continue
				}
				regexRule := RelationshipRule{
					KindA:         strings.ToLower(gvrName),
					KindB:         rule.KindB,
					MatchCriteria: append([]MatchCriterion{}, rule.MatchCriteria...),
					Relationship:  RelationshipType(fmt.Sprintf("%s_%s", rule.Relationship, strings.ToUpper(gvrName))),
				}
				counter++
				relationshipRules = append(relationshipRules, regexRule)
			}
			continue
		}
		// Add to global relationships
		counter++
		AddRelationshipRule(rule)
	}

	return counter, nil
}

func GetRelationships() map[string][]string {
	relationshipsMutex.RLock()
	defer relationshipsMutex.RUnlock()

	return relationships
}

func findRelationshipRulesBetweenKinds(kindA, kindB string) []RelationshipRule {
	var matchingRules []RelationshipRule
	for _, rule := range relationshipRules {
		// Check direct match (order matters)
		if strings.EqualFold(rule.KindA, kindA) && strings.EqualFold(rule.KindB, kindB) {
			matchingRules = append(matchingRules, rule)
		}
		// Check reverse match (order matters)
		if strings.EqualFold(rule.KindA, kindB) && strings.EqualFold(rule.KindB, kindA) {
			matchingRules = append(matchingRules, rule)
		}
	}

	// Sort rules by priority:
	// 1. Hardcoded/special relationships first (those not ending with _BY_PARENT_FIELD)
	// 2. Dynamically discovered relationships last
	slices.SortFunc(matchingRules, func(a, b RelationshipRule) int {
		aIsDynamic := strings.HasSuffix(string(a.Relationship), "_BY_PARENT_FIELD")
		bIsDynamic := strings.HasSuffix(string(b.Relationship), "_BY_PARENT_FIELD")

		if aIsDynamic && !bIsDynamic {
			return 1 // a comes after b
		}
		if !aIsDynamic && bIsDynamic {
			return -1 // a comes before b
		}
		return 0 // maintain relative order
	})

	return matchingRules
}

// Helper function to check if two relationship rules relate to the same resources
// without requiring exact type name matching
func areRelatedRules(ruleA, ruleB RelationshipRule, provider provider.Provider) bool {
	// If the kinds and direction are the same, they are related
	if (strings.EqualFold(ruleA.KindA, ruleB.KindA) && strings.EqualFold(ruleA.KindB, ruleB.KindB)) ||
		(strings.EqualFold(ruleA.KindA, ruleB.KindB) && strings.EqualFold(ruleA.KindB, ruleB.KindA)) {
		return true
	}

	// Try to resolve the GVRs for both rules
	// This will handle cases where one rule uses "configmap" and another uses "configmaps"
	// since the Kubernetes API will resolve both to the same GVR
	kindA1, _ := provider.FindGVR(ruleA.KindA)
	kindB1, _ := provider.FindGVR(ruleA.KindB)
	kindA2, _ := provider.FindGVR(ruleB.KindA)
	kindB2, _ := provider.FindGVR(ruleB.KindB)

	// If the resolved GVRs match in any direction, the rules are related
	return (kindA1 == kindA2 && kindB1 == kindB2) || (kindA1 == kindB2 && kindB1 == kindA2)
}

// Consolidate matching criteria from multiple rules into a single rule
func consolidateMatchingRules(rules []RelationshipRule, provider provider.Provider) RelationshipRule {
	if len(rules) == 0 {
		return RelationshipRule{}
	}

	// Start with the first rule (highest priority one)
	consolidated := rules[0]

	// For each additional rule, check if it relates to the same resources
	for i := 1; i < len(rules); i++ {
		// Use the provider to determine if the rules connect the same resources
		if areRelatedRules(consolidated, rules[i], provider) {
			// Add each criterion from the similar rule if it's not already present
			for _, crit := range rules[i].MatchCriteria {
				alreadyExists := false
				for _, existingCrit := range consolidated.MatchCriteria {
					if existingCrit.FieldA == crit.FieldA &&
						existingCrit.FieldB == crit.FieldB &&
						existingCrit.ComparisonType == crit.ComparisonType {
						alreadyExists = true
						break
					}
				}
				if !alreadyExists {
					consolidated.MatchCriteria = append(consolidated.MatchCriteria, crit)
				}
			}
		}
	}

	return consolidated
}

func (q *QueryExecutor) processRelationship(rel *Relationship, c *MatchClause, results *QueryResult, filteredResults map[string][]map[string]interface{}) (bool, error) {
	debugLog(fmt.Sprintf("Processing relationship: %+v\n", rel))

	// Determine relationship type and fetch related resources
	var relType RelationshipType

	// Resolve kinds if needed
	if rel.LeftNode.ResourceProperties.Kind == "" || rel.RightNode.ResourceProperties.Kind == "" {
		// Try to resolve the kind using relationships
		potentialKinds, err := FindPotentialKindsIntersection(c.Relationships, q.provider)
		if err != nil {
			return false, fmt.Errorf("unable to determine kind for nodes in relationship >> %s", err)
		}
		if len(potentialKinds) == 0 {
			return false, fmt.Errorf("unable to determine kind for nodes in relationship")
		}
		if len(potentialKinds) > 1 {
			// Instead of expanding the query here, we'll let rewriteQueryForKindlessNodes handle it
			return false, &QueryExpandedError{ExpandedQuery: "needs_rewrite"}
		}
		if rel.LeftNode.ResourceProperties.Kind == "" {
			rel.LeftNode.ResourceProperties.Kind = potentialKinds[0]
		}
		if rel.RightNode.ResourceProperties.Kind == "" {
			rel.RightNode.ResourceProperties.Kind = potentialKinds[0]
		}
	}

	leftKind, err := q.findGVR(rel.LeftNode.ResourceProperties.Kind)
	if err != nil {
		return false, fmt.Errorf("error finding API resource >> %s", err)
	}
	rightKind, err := q.findGVR(rel.RightNode.ResourceProperties.Kind)
	if err != nil {
		return false, fmt.Errorf("error finding API resource >> %s", err)
	}

	// Namespace special case (handled first)
	if rightKind.Resource == "namespaces" || leftKind.Resource == "namespaces" {
		relType = NamespaceHasResource
	}

	// Find all possible rules between these kinds
	if relType == "" {
		matchingRules := findRelationshipRulesBetweenKinds(leftKind.Resource, rightKind.Resource)

		if len(matchingRules) == 0 {
			// No relationship type found, error out
			return false, fmt.Errorf("relationship type not found between %s and %s", leftKind.Resource, rightKind.Resource)
		}

		// Check if we have multiple rules to consolidate
		if len(matchingRules) > 1 {
			// Consolidate criteria from related rules
			consolidatedRule := consolidateMatchingRules(matchingRules, q.provider)
			// Use the consolidated rule's relationship type
			relType = consolidatedRule.Relationship

			// Replace the rule in relationshipRules (temporarily, just for this query)
			for i, rule := range relationshipRules {
				if rule.Relationship == relType {
					// Create a copy to avoid modifying the global rule
					tempRule := relationshipRules[i]
					tempRule.MatchCriteria = consolidatedRule.MatchCriteria
					relationshipRules[i] = tempRule
					break
				}
			}
		} else {
			// Just one rule, use its relationship type
			relType = matchingRules[0].Relationship
		}

		debugLog("Selected relationship type %s from %d possible rules between %s and %s",
			relType, len(matchingRules), leftKind.Resource, rightKind.Resource)
	}

	rule, err := findRuleByRelationshipType(relType)
	if err != nil {
		return false, fmt.Errorf("error determining relationship type >> %s", err)
	}

	// Fetch and process related resources
	for _, node := range c.Nodes {
		if node.ResourceProperties.Name == rel.LeftNode.ResourceProperties.Name || node.ResourceProperties.Name == rel.RightNode.ResourceProperties.Name {
			if results.Data[node.ResourceProperties.Name] == nil {
				err := getNodeResources(node, q, c.ExtraFilters)
				if err != nil {
					return false, err
				}
			}
		}
	}

	var resourcesA, resourcesB []map[string]interface{}
	var filteredDirection Direction

	resultMapMutex.RLock()
	if rule.KindA == rightKind.Resource {
		resourcesA = getResourcesFromMap(filteredResults, rel.RightNode.ResourceProperties.Name)
		resourcesB = getResourcesFromMap(filteredResults, rel.LeftNode.ResourceProperties.Name)
		filteredDirection = Left
	} else if rule.KindA == leftKind.Resource {
		resourcesA = getResourcesFromMap(filteredResults, rel.LeftNode.ResourceProperties.Name)
		resourcesB = getResourcesFromMap(filteredResults, rel.RightNode.ResourceProperties.Name)
		filteredDirection = Right
	} else {
		resultMapMutex.RUnlock()
		return false, fmt.Errorf("relationship rule not found for %s and %s", rel.LeftNode.ResourceProperties.Kind, rel.RightNode.ResourceProperties.Kind)
	}
	resultMapMutex.RUnlock()

	matchedResources := applyRelationshipRule(resourcesA, resourcesB, rule, filteredDirection)

	// Add nodes and edges based on the matched resources
	rightResources := matchedResources["right"].([]map[string]interface{})
	leftResources := matchedResources["left"].([]map[string]interface{})

	// Add nodes
	for _, rightResource := range rightResources {
		if metadata, ok := rightResource["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				node := Node{
					Id:   rel.RightNode.ResourceProperties.Name,
					Kind: rightResource["kind"].(string),
					Name: name,
				}
				if node.Kind != "Namespace" {
					node.Namespace = getNamespaceName(metadata)
				}
				results.Graph.Nodes = append(results.Graph.Nodes, node)
			}
		}
	}

	for _, leftResource := range leftResources {
		if metadata, ok := leftResource["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				node := Node{
					Id:   rel.LeftNode.ResourceProperties.Name,
					Kind: leftResource["kind"].(string),
					Name: name,
				}
				if node.Kind != "Namespace" {
					node.Namespace = getNamespaceName(metadata)
				}
				results.Graph.Nodes = append(results.Graph.Nodes, node)
			}
		}
	}

	// Process edges
	for _, rightResource := range rightResources {
		for _, leftResource := range leftResources {
			// Check if these resources actually match according to the criteria
			for _, criterion := range rule.MatchCriteria {
				if matchByCriterion(rightResource, leftResource, criterion) || matchByCriterion(leftResource, rightResource, criterion) {
					rightNodeId := fmt.Sprintf("%s/%s", rightResource["kind"].(string), rightResource["metadata"].(map[string]interface{})["name"].(string))
					leftNodeId := fmt.Sprintf("%s/%s", leftResource["kind"].(string), leftResource["metadata"].(map[string]interface{})["name"].(string))
					results.Graph.Edges = append(results.Graph.Edges, Edge{
						From: rightNodeId,
						To:   leftNodeId,
						Type: string(relType),
					})
				}
			}
		}
	}

	filteredA := len(matchedResources["right"].([]map[string]interface{})) < len(resourcesA)
	filteredB := len(matchedResources["left"].([]map[string]interface{})) < len(resourcesB)

	filteredResults[rel.RightNode.ResourceProperties.Name] = matchedResources["right"].([]map[string]interface{})
	filteredResults[rel.LeftNode.ResourceProperties.Name] = matchedResources["left"].([]map[string]interface{})

	resultMapMutex.Lock()
	if resultMap[rel.RightNode.ResourceProperties.Name] != nil {
		if len(resultMap[rel.RightNode.ResourceProperties.Name].([]map[string]interface{})) > len(matchedResources["right"].([]map[string]interface{})) {
			resultMap[rel.RightNode.ResourceProperties.Name] = matchedResources["right"]
		}
	} else {
		resultMap[rel.RightNode.ResourceProperties.Name] = matchedResources["right"]
	}
	if resultMap[rel.LeftNode.ResourceProperties.Name] != nil {
		if len(resultMap[rel.LeftNode.ResourceProperties.Name].([]map[string]interface{})) > len(matchedResources["left"].([]map[string]interface{})) {
			resultMap[rel.LeftNode.ResourceProperties.Name] = matchedResources["left"]
		}
	} else {
		resultMap[rel.LeftNode.ResourceProperties.Name] = matchedResources["left"]
	}
	resultMapMutex.Unlock()

	return filteredA || filteredB, nil
}

// Move findExistingRelationshipRule to use a cache of GVR resolutions
func findExistingRelationshipRule(kindA, kindB string, gvrCache map[string]schema.GroupVersionResource) (int, bool) {
	// Get GVRs from cache instead of making API calls
	kindAGVR, okA := gvrCache[kindA]
	kindBGVR, okB := gvrCache[kindB]

	if !okA || !okB {
		return -1, false
	}

	for i, rule := range relationshipRules {
		// Skip special rules like NAMESPACE_HAS_RESOURCE
		if rule.Relationship == NamespaceHasResource {
			continue
		}

		// Get rule GVRs from cache
		ruleKindAGVR, okRA := gvrCache[rule.KindA]
		ruleKindBGVR, okRB := gvrCache[rule.KindB]

		if !okRA || !okRB {
			continue
		}

		// Check if this rule connects the same resource kinds (in same order)
		if ruleKindAGVR == kindAGVR && ruleKindBGVR == kindBGVR {
			return i, true
		}
	}

	return -1, false
}
