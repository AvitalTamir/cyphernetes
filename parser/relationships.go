package parser

import (
	"github.com/oliveagle/jsonpath"
)

type ResourceRelationship struct {
	FromKind string
	ToKind   string
	Type     RelationshipType
}

type RelationshipType string

const (
	DeployOwnRs RelationshipType = "DEPLOY_OWN_RS"
	RsOwnPod    RelationshipType = "RS_OWN_POD"
	StsOwnPod   RelationshipType = "STS_OWN_POD"
	DsOwnOwnPod RelationshipType = "DS_OWN_OWN_POD"
	JobOwnPod   RelationshipType = "JOB_OWN_POD"
	// services to pods
	Expose RelationshipType = "EXPOSE"
	// This is for configMaps, Volumes, Secrets in pods
	Mount RelationshipType = "MOUNT"
	// ingresses to services
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
	{
		KindA:        "pods",
		KindB:        "statefulsets",
		Relationship: StsOwnPod,
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
		KindB:        "daemonsets",
		Relationship: DsOwnOwnPod,
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
		KindB:        "jobs",
		Relationship: JobOwnPod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "metadata.ownerReferences",
				FieldB:         "metadata.name",
				ComparisonType: OwnerRefMatch,
			},
		},
	},
	{
		KindA:        "ingresses",
		KindB:        "services",
		Relationship: Route,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.rules.http.paths.backend.service.name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
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
			if !ok {
				logDebug("No resources found for selector: ", selector)
			}
			if !matchLabels(labels, selector) {
				return false
			}
		case ExactMatch:
			// Specific logic for field matching
			// use jsonpath to extract the fields

			// extract the fields
			fieldsA, err := jsonpath.JsonPathLookup(resourceA, criterion.FieldA)
			if err != nil {
				logDebug("Error extracting fieldA: ", err)
				return false
			}
			fieldsB, err := jsonpath.JsonPathLookup(resourceB, criterion.FieldB)
			if err != nil {
				logDebug("Error extracting fieldB: ", err)
				return false
			}
			if !matchFields(fieldsA, fieldsB) {
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
