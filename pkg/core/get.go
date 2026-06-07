package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AvitalTamir/jsonpath"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func getResourcesFromMap(filteredResults map[string][]map[string]interface{}, key string, state *executionState) []map[string]interface{} {
	if filtered, ok := filteredResults[key]; ok {
		return filtered
	}

	if resources, ok := state.getResources(key); ok {
		return resources
	}
	return nil
}

func (q *QueryExecutor) processNodes(c *MatchClause, results *QueryResult, state *executionState) error {
	debugLog("Processing nodes, current graph nodes: %+v\n", results.Graph.Nodes)

	// Track seen nodes for deduplication
	seenNodes := make(map[string]bool)
	for _, existingNode := range results.Graph.Nodes {
		nodeKey := fmt.Sprintf("%s/%s", existingNode.Kind, existingNode.Name)
		seenNodes[nodeKey] = true
	}

	for _, node := range c.Nodes {
		if node.ResourceProperties.Kind == "" {
			// Try to resolve the kind using relationships
			potentialKinds, err := FindPotentialKindsIntersection(c.Relationships, q.provider)
			if err != nil {
				return fmt.Errorf("unable to determine kind for node '%s' - no relationships found", node.ResourceProperties.Name)
			}
			if len(potentialKinds) == 0 {
				return fmt.Errorf("unable to determine kind for node '%s' - no relationships found", node.ResourceProperties.Name)
			}
			if len(potentialKinds) > 1 {
				return fmt.Errorf("ambiguous kind for node '%s' - possible kinds: %v", node.ResourceProperties.Name, potentialKinds)
			}
			node.ResourceProperties.Kind = potentialKinds[0]
		}

		if _, ok := state.getResources(node.ResourceProperties.Name); !ok {
			err := getNodeResources(node, q, c.ExtraFilters, state)
			if err != nil {
				return fmt.Errorf("error getting node resources >> %s", err)
			}
		}
		resources, ok := state.getResources(node.ResourceProperties.Name)
		if !ok {
			return fmt.Errorf("node %s resources were not loaded", node.ResourceProperties.Name)
		}
		for _, resource := range resources {
			metadata, err := getResourceMetadata(resource)
			if err != nil {
				return fmt.Errorf("error reading resource metadata for node %s: %w", node.ResourceProperties.Name, err)
			}
			kind, err := getResourceKind(resource)
			if err != nil {
				return fmt.Errorf("error reading resource kind for node %s: %w", node.ResourceProperties.Name, err)
			}
			name, err := getResourceName(metadata)
			if err != nil {
				return fmt.Errorf("error reading resource name for node %s: %w", node.ResourceProperties.Name, err)
			}
			newNode := Node{
				Id:   node.ResourceProperties.Name,
				Kind: kind,
				Name: name,
			}
			if newNode.Kind != "Namespace" {
				newNode.Namespace = getNamespaceName(metadata)
			}

			// Check if we've seen this node before
			nodeKey := fmt.Sprintf("%s/%s/%s", newNode.Kind, newNode.Namespace, newNode.Name)
			if !seenNodes[nodeKey] {
				seenNodes[nodeKey] = true
				debugLog("Adding new unique node from processNodes: %+v with key: %s\n", newNode, nodeKey)
				results.Graph.Nodes = append(results.Graph.Nodes, newNode)
			} else {
				debugLog("Skipping duplicate node in processNodes: %+v with key: %s\n", newNode, nodeKey)
			}
		}
	}
	debugLog("After processNodes, graph nodes: %+v\n", results.Graph.Nodes)
	return nil
}

func (q *QueryExecutor) findGVR(kind string) (schema.GroupVersionResource, error) {
	return tryResolveGVR(q.provider, kind)
}

func (q *QueryExecutor) providerKind(kind string) (string, error) {
	_, err := q.provider.FindGVR(kind)
	if err == nil {
		return kind, nil
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		return "", err
	}
	gvr, err := tryResolveGVR(q.provider, kind)
	if err != nil {
		return "", err
	}
	if gvr.Group == "" {
		return "core." + gvr.Resource, nil
	}
	return gvr.Resource + "." + gvr.Group, nil
}

func getNodeResources(n *NodePattern, q *QueryExecutor, extraFilters []*Filter, state *executionState) (err error) {
	namespace := state.namespace

	// Create a copy of ResourceProperties
	resourcePropertiesCopy := &ResourceProperties{}
	if n.ResourceProperties.Properties != nil {
		resourcePropertiesCopy.Properties = &Properties{
			PropertyList: make([]*Property, len(n.ResourceProperties.Properties.PropertyList)),
		}
		copy(resourcePropertiesCopy.Properties.PropertyList, n.ResourceProperties.Properties.PropertyList)
	}

	if resourcePropertiesCopy.Properties != nil && len(resourcePropertiesCopy.Properties.PropertyList) > 0 {
		for i, prop := range resourcePropertiesCopy.Properties.PropertyList {
			if prop.Key == "namespace" || prop.Key == "metadata.namespace" {
				namespace = prop.Value.(string)
				// Remove the namespace slice from the properties
				resourcePropertiesCopy.Properties.PropertyList = append(resourcePropertiesCopy.Properties.PropertyList[:i], resourcePropertiesCopy.Properties.PropertyList[i+1:]...)
			}
		}
	}

	var fieldSelector string
	var labelSelector string
	var hasNameSelector bool
	var hasLabelSelector bool

	if resourcePropertiesCopy.Properties != nil {
		for _, prop := range resourcePropertiesCopy.Properties.PropertyList {
			if prop.Key == "name" || prop.Key == "metadata.name" || prop.Key == `"name"` || prop.Key == `"metadata.name"` {
				fieldSelector += fmt.Sprintf("metadata.name=%s,", prop.Value)
				hasNameSelector = true
			} else {
				hasLabelSelector = true
				labelSelector += fmt.Sprintf("%s=%s,", prop.Key, prop.Value)
			}
		}
		fieldSelector = strings.TrimSuffix(fieldSelector, ",")
		labelSelector = strings.TrimSuffix(labelSelector, ",")
	}
	if hasNameSelector && hasLabelSelector {
		// both name and label selectors are specified, error out
		return fmt.Errorf("the 'name' selector can be used by itself or combined with 'namespace', but not with other label selectors")
	}

	// Check if the resource has already been fetched for this exact query shape.
	cacheKey, err := q.resourceFetchKey(n, namespace, fieldSelector, labelSelector, extraFilters)
	if err != nil {
		return fmt.Errorf("error getting resource fetch key: %v", err)
	}

	if cachedResult, ok := state.cachedResources(cacheKey); !ok {
		// Get resources using the provider
		providerKind, err := q.providerKind(n.ResourceProperties.Kind)
		if err != nil {
			return fmt.Errorf("error resolving resource kind: %v", err)
		}
		resources, err := q.provider.GetK8sResources(providerKind, fieldSelector, labelSelector, namespace)
		if err != nil {
			return fmt.Errorf("error getting resources: %v", err)
		}

		// Apply extra filters from WHERE clause
		resourceList, ok := resources.([]map[string]interface{})
		if !ok {
			return fmt.Errorf("provider returned %T for %s, expected []map[string]interface{}", resources, n.ResourceProperties.Kind)
		}
		var filtered []map[string]interface{}

		var subMatches []*SubMatch

		// Process each resource
		for _, resource := range resourceList {
			keep := true
			// Apply extra filters
			for _, extraFilter := range extraFilters {
				if extraFilter.Type == "KeyValuePair" {
					filter := extraFilter.KeyValuePair
					// Extract node name from filter key
					var resultMapKey string
					dotIndex := strings.Index(filter.Key, ".")
					if dotIndex != -1 {
						resultMapKey = filter.Key[:dotIndex]
					} else {
						resultMapKey = filter.Key
					}

					// Handle escaped dots
					for strings.HasSuffix(resultMapKey, "\\") {
						nextDotIndex := strings.Index(filter.Key[len(resultMapKey)+1:], ".")
						if nextDotIndex == -1 {
							resultMapKey = filter.Key
							break
						}
						resultMapKey = filter.Key[:len(resultMapKey)+1+nextDotIndex]
					}

					if resultMapKey == n.ResourceProperties.Name {
						// Transform path
						path := filter.Key
						path = strings.Replace(path, resultMapKey+".", "$.", 1)

						// Compile and fix the path
						compiledPath, err := jsonpath.Compile(path)
						if err != nil {
							keep = false
							break
						}
						compiledPath = fixCompiledPath(compiledPath)

						debugLog("Looking up path: %s in resource: %+v", path, resource)

						// If path contains wildcards, we need special handling
						if strings.Contains(path, "[*]") {
							keep = evaluateWildcardPath(resource, path, filter.Value, filter.Operator)
							if filter.IsNegated {
								keep = !keep
							}
						} else {
							// Regular path handling using the fixed compiled path
							value, err := compiledPath.Lookup(resource)
							if err != nil {
								keep = false
								break
							}

							// Check if the filter value is a temporal expression
							if temporalExpr, ok := filter.Value.(*TemporalExpression); ok {
								// Convert resource value to time.Time if it's a string
								var resourceTime time.Time
								if timeStr, ok := value.(string); ok {
									resourceTime, err = time.Parse(time.RFC3339, timeStr)
									if err != nil {
										keep = false
										break
									}
								} else {
									keep = false
									break
								}

								// Use temporal handler to compare values
								temporalHandler := NewTemporalHandler()
								keep, err = temporalHandler.CompareTemporalValues(resourceTime, temporalExpr, filter.Operator)
								if err != nil {
									keep = false
									break
								}
							} else {
								// Regular value comparison
								resourceValue, filterValue, err := convertToComparableTypes(value, filter.Value)
								if err != nil {
									keep = false
									break
								}

								keep = compareValues(resourceValue, filterValue, filter.Operator)
							}

							if filter.IsNegated {
								keep = !keep
							}
						}

						if !keep {
							break
						}
					}
				} else if extraFilter.Type == "SubMatch" {
					subMatches = append(subMatches, extraFilter.SubMatch)
				}
			}

			if keep {
				filtered = append(filtered, resource)
			}
		}

		for _, subMatch := range subMatches {
			if subMatch.ReferenceNodeName != n.ResourceProperties.Name {
				continue
			}

			// for each submatch, run checkSubMatch
			subMatchResults, err := q.checkSubMatch(subMatch, n.ResourceProperties.Name, state)
			if err != nil {
				return fmt.Errorf("error checking submatch: %v", err)
			}

			// find the delta between subMatchResults and filtered
			// if subMatch.IsNegated, discard the delta
			// if not negated, keep the delta
			var newFiltered []map[string]interface{}
			// Get matched resources for this node if they exist
			matchedResources, hasMatches := subMatchResults["_ref_"+n.ResourceProperties.Name]

			for _, resource := range filtered {
				isMatch := hasMatches && isResourceInList(resource, matchedResources)
				// For negated patterns, keep resources that DON'T match
				// For non-negated patterns, keep resources that DO match
				if isMatch != subMatch.IsNegated {
					newFiltered = append(newFiltered, resource)
				}
			}
			filtered = newFiltered
		}

		state.cacheResources(cacheKey, n.ResourceProperties.Name, filtered)
	} else {
		// If we found it in cache, just copy to resultMap
		state.setResources(n.ResourceProperties.Name, cachedResult)
	}

	return nil
}

func compareValues(resourceValue, filterValue interface{}, operator string) bool {
	switch operator {
	case "EQUALS", "=", "==":
		return resourceValue == filterValue
	case "NOT_EQUALS", "!=":
		return resourceValue != filterValue
	case "GREATER_THAN", ">":
		if rv, ok := resourceValue.(float64); ok {
			if fv, ok := filterValue.(float64); ok {
				return rv > fv
			}
		}
	case "LESS_THAN", "<":
		if rv, ok := resourceValue.(float64); ok {
			if fv, ok := filterValue.(float64); ok {
				return rv < fv
			}
		}
	case "GREATER_THAN_EQUALS", ">=":
		if rv, ok := resourceValue.(float64); ok {
			if fv, ok := filterValue.(float64); ok {
				return rv >= fv
			}
		}
	case "LESS_THAN_EQUALS", "<=":
		if rv, ok := resourceValue.(float64); ok {
			if fv, ok := filterValue.(float64); ok {
				return rv <= fv
			}
		}
	case "CONTAINS":
		strA := fmt.Sprintf("%v", resourceValue)
		strB := fmt.Sprintf("%v", filterValue)
		return strings.Contains(strA, strB)
	case "REGEX_COMPARE":
		if filterValueStr, ok := filterValue.(string); ok {
			if resultValueStr, ok := resourceValue.(string); ok {
				if regex, err := regexp.Compile(filterValueStr); err == nil {
					return regex.MatchString(resultValueStr)
				}
			}
		}
	}
	return false
}

func (q *QueryExecutor) checkSubMatch(subMatch *SubMatch, referenceNodeName string, state *executionState) (map[string][]map[string]interface{}, error) {
	subMatch = cloneSubMatch(subMatch)
	subState := newExecutionState()
	subState.namespace = state.namespace

	// Create temporary results and filtered results maps
	tempResults := QueryResult{
		Data: make(map[string]interface{}),
	}
	filteredResults := make(map[string][]map[string]interface{})

	for _, node := range subMatch.Nodes {
		if node.ResourceProperties.Name == referenceNodeName {
			node.ResourceProperties.Name = "_ref_" + node.ResourceProperties.Name
			break
		}
	}

	// also rename this node in the relationships
	for _, rel := range subMatch.Relationships {
		if rel.LeftNode.ResourceProperties.Name == referenceNodeName {
			rel.LeftNode.ResourceProperties.Name = "_ref_" + rel.LeftNode.ResourceProperties.Name
		}
		if rel.RightNode.ResourceProperties.Name == referenceNodeName {
			rel.RightNode.ResourceProperties.Name = "_ref_" + rel.RightNode.ResourceProperties.Name
		}
	}

	// Create a match clause for relationship processing
	matchClause := &MatchClause{
		Nodes:         subMatch.Nodes,
		Relationships: subMatch.Relationships,
	}

	// Get resources for the reference node first
	for _, node := range matchClause.Nodes {
		if node.ResourceProperties.Name == "_ref_"+subMatch.ReferenceNodeName {
			err := getNodeResources(node, q, nil, subState)
			if err != nil {
				return nil, err
			}
			// Initialize filteredResults and resultMap with the reference node's resources
			filteredResults[node.ResourceProperties.Name] = getResourcesFromMap(filteredResults, node.ResourceProperties.Name, subState)
			break
		}
	}

	// Process each relationship in the pattern, with multiple passes if needed
	for i := 0; i < len(matchClause.Relationships)*2; i++ {
		filteringOccurred := false

		for _, rel := range matchClause.Relationships {
			// Get resources for nodes that aren't the reference node
			if rel.LeftNode.ResourceProperties.Name != "_ref_"+subMatch.ReferenceNodeName {
				err := getNodeResources(rel.LeftNode, q, nil, subState)
				if err != nil {
					return nil, err
				}
			}
			if rel.RightNode.ResourceProperties.Name != "_ref_"+subMatch.ReferenceNodeName {
				err := getNodeResources(rel.RightNode, q, nil, subState)
				if err != nil {
					return nil, err
				}
			}

			// Process the relationship and update filteredResults
			filtered, err := q.processRelationship(rel, matchClause, &tempResults, filteredResults, subState)
			if err != nil {
				return nil, err
			}
			filteringOccurred = filteringOccurred || filtered

			// Check if either side of the relationship has no results
			leftResults := filteredResults[rel.LeftNode.ResourceProperties.Name]
			rightResults := filteredResults[rel.RightNode.ResourceProperties.Name]
			if len(leftResults) == 0 || len(rightResults) == 0 {
				// If any part of the chain has no results, the entire pattern doesn't match
				return make(map[string][]map[string]interface{}), nil
			}
		}

		if !filteringOccurred {
			break
		}

		// Update resultMap with filtered results for the next pass
		for k, v := range filteredResults {
			subState.setResources(k, v)
		}
	}

	return filteredResults, nil
}

func isResourceInList(resource map[string]interface{}, matchedResources []map[string]interface{}) bool {
	resourceJSON, _ := json.Marshal(resource)
	for _, matchedResource := range matchedResources {
		matchedJSON, _ := json.Marshal(matchedResource)
		if string(resourceJSON) == string(matchedJSON) {
			return true
		}
	}
	return false
}

func extractKindFromSchemaName(schemaName string) string {
	parts := strings.Split(schemaName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
