package core

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type executionState struct {
	mu          sync.RWMutex
	resultMap   map[string]interface{}
	resultCache map[string]interface{}
	matchNodes  []*NodePattern
	namespace   string
	dryRun      bool
	graphNodes  map[string]bool
	graphEdges  map[string]bool
	hasPatterns bool
}

func newExecutionState() *executionState {
	return &executionState{
		resultMap:   make(map[string]interface{}),
		resultCache: make(map[string]interface{}),
		graphNodes:  make(map[string]bool),
		graphEdges:  make(map[string]bool),
	}
}

func (s *executionState) getResources(key string) ([]map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return resourcesFromValue(s.resultMap[key])
}

func (s *executionState) setResources(key string, resources []map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resultMap[key] = resources
}

func (s *executionState) deleteResources(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.resultMap, key)
}

func (s *executionState) cachedResources(key string) ([]map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return resourcesFromValue(s.resultCache[key])
}

func (s *executionState) cacheResources(cacheKey, resultKey string, resources []map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resultCache[cacheKey] = resources
	s.resultMap[resultKey] = resources
}

func (s *executionState) copyCachedResources(cacheKey, resultKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	resources, ok := resourcesFromValue(s.resultCache[cacheKey])
	if !ok {
		return false
	}
	s.resultMap[resultKey] = resources
	return true
}

func (s *executionState) setResourcesIfMoreSelective(key string, resources []map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := resourcesFromValue(s.resultMap[key])
	if !ok || len(current) > len(resources) {
		s.resultMap[key] = resources
	}
}

func (s *executionState) markPatternRows() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasPatterns = true
}

func (s *executionState) hasPatternRows() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasPatterns
}

func resourcesFromValue(value interface{}) ([]map[string]interface{}, bool) {
	resources, ok := value.([]map[string]interface{})
	return resources, ok
}

func requireResourceList(value interface{}, node string) ([]map[string]interface{}, error) {
	resources, ok := resourcesFromValue(value)
	if !ok {
		return nil, fmt.Errorf("expected resources for node %s, got %T", node, value)
	}
	return resources, nil
}

func filterSignature(filters []*Filter) string {
	if len(filters) == 0 {
		return ""
	}
	parts := make([]string, 0, len(filters))
	for _, filter := range filters {
		if filter == nil {
			continue
		}
		switch filter.Type {
		case "KeyValuePair":
			if filter.KeyValuePair != nil {
				kvp := filter.KeyValuePair
				parts = append(parts, fmt.Sprintf("kv:%s:%s:%t:%#v", kvp.Key, kvp.Operator, kvp.IsNegated, kvp.Value))
			}
		case "SubMatch":
			if filter.SubMatch != nil {
				parts = append(parts, fmt.Sprintf("sub:%s:%t:%d:%d", filter.SubMatch.ReferenceNodeName, filter.SubMatch.IsNegated, len(filter.SubMatch.Nodes), len(filter.SubMatch.Relationships)))
			}
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func cloneSubMatch(subMatch *SubMatch) *SubMatch {
	if subMatch == nil {
		return nil
	}
	nodes := make([]*NodePattern, len(subMatch.Nodes))
	nodesByName := make(map[string]*NodePattern, len(subMatch.Nodes))
	for i, node := range subMatch.Nodes {
		nodes[i] = cloneNodePattern(node)
		if nodes[i] != nil && nodes[i].ResourceProperties != nil {
			nodesByName[nodes[i].ResourceProperties.Name] = nodes[i]
		}
	}

	relationships := make([]*Relationship, len(subMatch.Relationships))
	for i, rel := range subMatch.Relationships {
		relationships[i] = cloneRelationship(rel, nodesByName)
	}

	return &SubMatch{
		IsNegated:         subMatch.IsNegated,
		Nodes:             nodes,
		Relationships:     relationships,
		ReferenceNodeName: subMatch.ReferenceNodeName,
	}
}

func cloneRelationship(rel *Relationship, nodesByName map[string]*NodePattern) *Relationship {
	if rel == nil {
		return nil
	}
	left := cloneNodePattern(rel.LeftNode)
	right := cloneNodePattern(rel.RightNode)
	if rel.LeftNode != nil && rel.LeftNode.ResourceProperties != nil {
		if node, ok := nodesByName[rel.LeftNode.ResourceProperties.Name]; ok {
			left = node
		}
	}
	if rel.RightNode != nil && rel.RightNode.ResourceProperties != nil {
		if node, ok := nodesByName[rel.RightNode.ResourceProperties.Name]; ok {
			right = node
		}
	}
	return &Relationship{
		ResourceProperties: cloneResourceProperties(rel.ResourceProperties),
		Direction:          rel.Direction,
		LeftNode:           left,
		RightNode:          right,
	}
}

func cloneNodePattern(node *NodePattern) *NodePattern {
	if node == nil {
		return nil
	}
	return &NodePattern{
		ResourceProperties: cloneResourceProperties(node.ResourceProperties),
		IsAnonymous:        node.IsAnonymous,
	}
}

func cloneResourceProperties(props *ResourceProperties) *ResourceProperties {
	if props == nil {
		return nil
	}
	cloned := &ResourceProperties{
		Name:     props.Name,
		Kind:     props.Kind,
		JsonData: props.JsonData,
	}
	if props.Properties != nil {
		cloned.Properties = &Properties{
			PropertyList: make([]*Property, len(props.Properties.PropertyList)),
		}
		for i, prop := range props.Properties.PropertyList {
			if prop == nil {
				continue
			}
			cloned.Properties.PropertyList[i] = &Property{
				Key:   prop.Key,
				Value: prop.Value,
			}
		}
	}
	return cloned
}
