package cmd

type ResourceRelationship struct {
	FromKind string
	ToKind   string
	Type     RelationshipType
}

type RelationshipType string

const (
	Own    RelationshipType = "OWN"
	Expose RelationshipType = "EXPOSE"
	// This is for configMaps, Volumes, Secrets in pods
	Mount RelationshipType = "MOUNT"
	// ingress and service:
	Route RelationshipType = "ROUTE"
)

type ComparisonType string

const (
	ExactMatch    ComparisonType = "ExactMatch"
	OwnerRefMatch ComparisonType = "OwnerRefMatch"
)

type MatchCriterion struct {
	FieldA         string
	FieldB         string
	ComparisonType ComparisonType
}

type RelationshipRule struct {
	KindA         string
	KindB         string
	Relationship  RelationshipType
	MatchCriteria []MatchCriterion
}

var relationshipRules = []RelationshipRule{

	{
		KindA:        "pods",
		KindB:        "replicasets",
		Relationship: Own,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "metadata.ownerReferences",
				FieldB:         "metadata.name",
				ComparisonType: OwnerRefMatch,
			},
		},
	},
	// Add more rules here...
}

func findRuleByRelationshipType(relationshipType RelationshipType) RelationshipRule {
	for _, rule := range relationshipRules {
		if rule.Relationship == relationshipType {
			return rule
		}
	}
	return RelationshipRule{}
}

func matchByCriteria(resourceA, resourceB interface{}, criteria []MatchCriterion) bool {
	for _, criterion := range criteria {
		switch criterion.ComparisonType {
		case ExactMatch:
			// Implement exact match logic (e.g., label selectors)
		case OwnerRefMatch:
			// Specific logic for owner reference matching
			ownerRefs, ok := resourceA.(map[string]interface{})["metadata"].(map[string]interface{})["ownerReferences"].([]interface{})
			if !ok {
				continue
			}
			nameB, ok := resourceB.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)
			if !ok {
				continue
			}
			if !ok {
				return false
			}
			if !matchOwnerReferences(ownerRefs, nameB) {
				return false
			}
			// Add more cases as needed
		}
	}
	return true
}

func matchOwnerReferences(ownerRefs []interface{}, name string) bool {
	for _, ref := range ownerRefs {
		refMap, ok := ref.(map[string]interface{})
		if !ok {
			continue
		}
		if refMap["name"] == name {
			return true
		}
	}
	return false
}

func applyRelationshipRule(resourcesA, resourcesB []map[string]interface{}, rule RelationshipRule) []map[string]interface{} {
	var matchedResources []map[string]interface{}
	for _, resourceA := range resourcesA {
		for _, resourceB := range resourcesB {
			if matchByCriteria(resourceA, resourceB, rule.MatchCriteria) {
				matchedResources = append(matchedResources, resourceB)
			}
		}
	}
	return matchedResources
}
