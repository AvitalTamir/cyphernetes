package parser

import (
	"fmt"
	"strings"

	"github.com/oliveagle/jsonpath"
)

type ResourceRelationship struct {
	FromKind string
	ToKind   string
	Type     RelationshipType
}

type RelationshipType string

const (
	// deployments to replicasets / replicasets to pods / statefulsets to pods / daemonsets to pods etc.
	DeploymentOwnReplicaset RelationshipType = "DEPLOYMENT_OWN_REPLICASET"
	ReplicasetOwnPod        RelationshipType = "REPLICASET_OWN_POD"
	StatefulsetOwnPod       RelationshipType = "STATEFULSET_OWN_POD"
	DaemonsetOwnPod         RelationshipType = "DAEMONSET_OWN_POD"
	JobOwnPod               RelationshipType = "JOB_OWN_POD"

	// services to pods / deployments / statefulsets / daemonsets / replicasets
	ServiceExposePod         RelationshipType = "SERVICE_EXPOSE_POD"
	ServiceExposeDeployment  RelationshipType = "SERVICE_EXPOSE_DEPLOYMENT"
	ServiceExposeStatefulset RelationshipType = "SERVICE_EXPOSE_STATEFULSET"
	ServiceExposeDaemonset   RelationshipType = "SERVICE_EXPOSE_DAEMONSET"
	ServiceExposeReplicaset  RelationshipType = "SERVICE_EXPOSE_REPLICASET"

	// This is for configMaps, Volumes, Secrets in pods
	Mount RelationshipType = "MOUNT"
	// ingresses to services
	Route RelationshipType = "ROUTE"

	// special relationships
	NamespaceHasResource RelationshipType = "NAMESPACE_HAS_RESOURCE"
)

type ComparisonType string

const (
	ExactMatch    ComparisonType = "ExactMatch"
	OwnerRefMatch ComparisonType = "OwnerRefMatch"
	ContainsAll   ComparisonType = "ContainsAll"
)

type MatchCriterion struct {
	FieldA         string
	FieldB         string
	ComparisonType ComparisonType
	DefaultProps   []DefaultProp
}

type DefaultProp struct {
	FieldA  string
	FieldB  string
	Default interface{}
}

type RelationshipRule struct {
	KindA        string
	KindB        string
	Relationship RelationshipType
	// Currently only supports one match criterion but can be extended to support multiple
	MatchCriteria []MatchCriterion
}

var relationshipRules = []RelationshipRule{

	{
		KindA:        "pods",
		KindB:        "replicasets",
		Relationship: ReplicasetOwnPod,
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
		Relationship: DeploymentOwnReplicaset,
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
		Relationship: ServiceExposePod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "metadata.labels",
				FieldB:         "spec.selector",
				ComparisonType: ContainsAll,
				DefaultProps: []DefaultProp{
					{
						FieldA:  "",
						FieldB:  "$.spec.ports[].port",
						Default: 80,
					},
				},
			},
		},
	},
	{
		KindA:        "pods",
		KindB:        "statefulsets",
		Relationship: StatefulsetOwnPod,
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
		Relationship: DaemonsetOwnPod,
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
				FieldA:         "$.spec.rules[].http.paths[].backend.service.name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
				DefaultProps: []DefaultProp{
					{
						FieldA:  "$.spec.rules[].http.paths[].pathType",
						FieldB:  "",
						Default: "ImplementationSpecific",
					},
					{
						FieldA:  "$.spec.rules[].http.paths[].path",
						FieldB:  "",
						Default: "/",
					},
					{
						FieldA:  "$.spec.rules[].http.paths[].backend.service.port.number",
						FieldB:  "",
						Default: 80,
					},
				},
			},
		},
	},
	{
		KindA:        "replicasets",
		KindB:        "services",
		Relationship: ServiceExposeReplicaset,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.template.metadata.labels",
				FieldB:         "$.spec.selector",
				ComparisonType: ContainsAll,
				DefaultProps: []DefaultProp{
					{
						FieldA:  "",
						FieldB:  "$.spec.ports[].port",
						Default: 80,
					},
				},
			},
		},
	},
	{
		KindA:        "statefulsets",
		KindB:        "services",
		Relationship: ServiceExposeStatefulset,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.template.metadata.labels",
				FieldB:         "$.spec.selector",
				ComparisonType: ContainsAll,
				DefaultProps: []DefaultProp{
					{
						FieldA:  "",
						FieldB:  "$.spec.ports[].port",
						Default: 80,
					},
				},
			},
		},
	},
	{
		KindA:        "daemonsets",
		KindB:        "services",
		Relationship: ServiceExposeDaemonset,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.template.metadata.labels",
				FieldB:         "$.spec.selector",
				ComparisonType: ContainsAll,
				DefaultProps: []DefaultProp{
					{
						FieldA:  "",
						FieldB:  "$.spec.ports[].port",
						Default: 80,
					},
				},
			},
		},
	},
	{
		KindA:        "deployments",
		KindB:        "services",
		Relationship: ServiceExposeDeployment,
		MatchCriteria: []MatchCriterion{
			{
				FieldA: "$.spec.template.metadata.labels",
				FieldB: "$.spec.selector",
				DefaultProps: []DefaultProp{
					{
						FieldA:  "",
						FieldB:  "$.spec.ports[].port",
						Default: 80,
					},
				},
				ComparisonType: ContainsAll,
			},
		},
	},
	{
		KindA:        "namespaces",
		KindB:        "*",
		Relationship: NamespaceHasResource,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.name",
				FieldB:         "$.metadata.namespace",
				ComparisonType: ExactMatch,
			},
		},
	},
	// Add more rules here...
}

func findRuleByRelationshipType(relationshipType RelationshipType) (RelationshipRule, error) {
	for _, rule := range relationshipRules {
		if rule.Relationship == relationshipType {
			return rule, nil
		}
	}
	return RelationshipRule{}, fmt.Errorf("rule not found for relationship type: %s", relationshipType)
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
