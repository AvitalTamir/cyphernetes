package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/avitaltamir/cyphernetes/pkg/provider"
)

// FindPotentialKinds returns all possible target kinds that could have a relationship with the given source kind
func FindPotentialKinds(sourceKind string, provider provider.Provider) ([]string, error) {
	sourceKind = strings.ToLower(sourceKind)
	debugLog("FindPotentialKinds: looking for relationships for sourceKind=%s", sourceKind)

	// Get the plural form using FindGVR
	gvr, err := tryResolveGVR(provider, sourceKind)
	if err != nil {
		return []string{}, fmt.Errorf("error getting GVR for %s: %v", sourceKind, err)
	}
	sourceKind = strings.ToLower(gvr.Resource)

	var sourceKindFull string
	if gvr.Group != "" {
		sourceKindFull = fmt.Sprintf("%s.%s", gvr.Resource, gvr.Group)
	} else {
		sourceKindFull = "core." + sourceKind
	}

	// Check cache with plural form
	potentialKindsMutex.RLock()
	if kinds, exists := potentialKindsCache[sourceKindFull]; exists {
		potentialKindsMutex.RUnlock()
		debugLog("FindPotentialKinds: found in cache for %s = %v", sourceKind, kinds)
		return kinds, nil
	}
	potentialKindsMutex.RUnlock()

	// If not in cache, fall back to scanning rules (this should be rare)
	debugLog("FindPotentialKinds: cache miss for %s, falling back to rule scan", sourceKind)
	potentialKinds := make(map[string]bool)
	rules := GetRelationshipRules()
	debugLog("FindPotentialKinds: found %d relationship rules", len(rules))

	for _, rule := range rules {
		// Get proper plural forms using FindGVR
		gvrA, err := provider.FindGVR(rule.KindA)
		if err != nil {
			debugLog("Error getting GVR for %s: %v", rule.KindA, err)
			continue
		}
		gvrB, err := provider.FindGVR(rule.KindB)
		if err != nil {
			debugLog("Error getting GVR for %s: %v", rule.KindB, err)
			continue
		}

		ruleKindA := strings.ToLower(gvrA.Resource)
		ruleKindB := strings.ToLower(gvrB.Resource)

		debugLog("FindPotentialKinds: checking rule KindA=%s, KindB=%s, Relationship=%s", ruleKindA, ruleKindB, rule.Relationship)
		if ruleKindB == sourceKind {
			debugLog("FindPotentialKinds: matched KindB, adding KindA=%s", ruleKindA)
			potentialKinds[ruleKindA] = true
		}
		if ruleKindA == sourceKind {
			debugLog("FindPotentialKinds: matched KindA, adding KindB=%s", ruleKindB)
			potentialKinds[ruleKindB] = true
		}
	}

	var result []string
	for kind := range potentialKinds {
		result = append(result, kind)
	}
	sort.Strings(result)

	// Update cache with results
	potentialKindsMutex.Lock()
	potentialKindsCache[sourceKind] = result
	potentialKindsMutex.Unlock()

	debugLog("FindPotentialKinds: final result for %s = %v", sourceKind, result)
	return result, nil
}

// FindPotentialKindsIntersection returns the intersection of possible kinds from multiple relationships
func FindPotentialKindsIntersection(relationships []*Relationship, provider provider.Provider) ([]string, error) {
	debugLog("FindPotentialKindsIntersection: Starting with relationships: %+v", relationships)
	if len(relationships) == 0 {
		debugLog("FindPotentialKindsIntersection: no relationships provided")
		return []string{}, nil
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
		return []string{}, nil
	}

	// Find all known kinds in the relationships
	knownKinds := make(map[string]bool)
	for _, rel := range relationships {
		if rel.LeftNode.ResourceProperties.Kind != "" {
			knownKinds[strings.ToLower(rel.LeftNode.ResourceProperties.Kind)] = true
			debugLog(fmt.Sprintf("FindPotentialKindsIntersection: Found known kind (left): %s", rel.LeftNode.ResourceProperties.Kind))
		}
		if rel.RightNode.ResourceProperties.Kind != "" {
			knownKinds[strings.ToLower(rel.RightNode.ResourceProperties.Kind)] = true
			debugLog(fmt.Sprintf("FindPotentialKindsIntersection: Found known kind (right): %s", rel.RightNode.ResourceProperties.Kind))
		}
	}
	debugLog(fmt.Sprintf("FindPotentialKindsIntersection: All known kinds: %v", knownKinds))

	// If no known kinds, return empty
	if len(knownKinds) == 0 {
		debugLog("FindPotentialKindsIntersection: no known kinds found")
		return []string{}, nil
	}

	// Initialize result with potential kinds from first known kind
	var firstKnownKind string
	for kind := range knownKinds {
		firstKnownKind = kind
		break
	}

	result := make(map[string]bool)
	initialPotentialKinds, err := FindPotentialKinds(firstKnownKind, provider)
	if err != nil {
		return nil, fmt.Errorf("%s", err)
	}
	debugLog(fmt.Sprintf("FindPotentialKindsIntersection: Initial potential kinds from %s: %v", firstKnownKind, initialPotentialKinds))
	for _, kind := range initialPotentialKinds {
		result[kind] = true
	}

	// For each additional known kind, intersect with its potential kinds
	for kind := range knownKinds {
		if kind == firstKnownKind {
			continue
		}

		potentialKinds, err := FindPotentialKinds(kind, provider)
		if err != nil {
			return nil, fmt.Errorf("unable to determine kind for nodes in relationship >> %s", err)
		}
		debugLog(fmt.Sprintf("FindPotentialKindsIntersection: Potential kinds for %s: %v", kind, potentialKinds))

		newResult := make(map[string]bool)
		// Keep only kinds that exist in both sets
		for _, potentialKind := range potentialKinds {
			if result[potentialKind] {
				debugLog(fmt.Sprintf("FindPotentialKindsIntersection: Keeping common kind %s", potentialKind))
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
	debugLog(fmt.Sprintf("FindPotentialKindsIntersection: Final result=%v", kinds))
	return kinds, nil
}

// Helper function to check if a string slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
