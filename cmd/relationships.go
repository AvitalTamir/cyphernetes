package cmd

type ResourceRelationship struct {
	FromKind string
	ToKind   string
	Type     RelationshipType
}

type RelationshipType string

const (
	DeployOwnRs RelationshipType = "DEPLOY_OWN_RS"
	RsOwnPod    RelationshipType = "RS_OWN_POD"
	Expose      RelationshipType = "EXPOSE"
	// This is for configMaps, Volumes, Secrets in pods
	Mount RelationshipType = "MOUNT"
	// ingress and service:
	Route RelationshipType = "ROUTE"
)

type ComparisonType string

const (
	ExactMatch    ComparisonType = "ExactMatch"
	OwnerRefMatch ComparisonType = "OwnerRefMatch"
	HasLabels     ComparisonType = "HasLabels"
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
		Relationship: RsOwnPod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "metadata.ownerReferences",
				FieldB:         "metadata.name",
				ComparisonType: OwnerRefMatch,
			},
		},
	},
	{
		KindA:        "replicasets",
		KindB:        "deployments",
		Relationship: DeployOwnRs,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "metadata.ownerReferences",
				FieldB:         "metadata.name",
				ComparisonType: OwnerRefMatch,
			},
		},
	},
	{
		KindA:        "pods",
		KindB:        "services",
		Relationship: Expose,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "metadata.labels",
				FieldB:         "spec.selector",
				ComparisonType: HasLabels,
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
			if !matchOwnerReferences(ownerRefs, nameB) {
				return false
			}
			// Add more cases as needed
		case HasLabels:
			// Specific logic for label matching
			labels, ok := resourceA.(map[string]interface{})["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
			if !ok {
				continue
			}
			selector, ok := resourceB.(map[string]interface{})["spec"].(map[string]interface{})["selector"].(map[string]interface{})
			if !matchLabels(labels, selector) {
				return false
			}
		}
	}
	return true
}

func matchLabels(labels, selector map[string]interface{}) bool {
	if len(selector) == 0 {
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

func applyRelationshipRule(resourcesA, resourcesB []map[string]interface{}, rule RelationshipRule, direction Direction) []map[string]interface{} {
	var matchedResources []map[string]interface{}
	for _, resourceA := range resourcesA {
		for _, resourceB := range resourcesB {
			if matchByCriteria(resourceA, resourceB, rule.MatchCriteria) {
				if direction == Left {
					matchedResources = append(matchedResources, resourceA)
				} else if direction == Right {
					matchedResources = append(matchedResources, resourceB)
				} else {
					return nil
				}
			}
		}
	}
	return matchedResources
}
