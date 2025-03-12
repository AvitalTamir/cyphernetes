package core

import (
	"fmt"
	"strings"
)

func ExecuteMultiContextQuery(ast *Expression, namespace string) (QueryResult, error) {
	if len(ast.Contexts) == 0 {
		return QueryResult{}, fmt.Errorf("no contexts provided for multi-context query")
	}

	// Initialize combined results
	combinedResults := QueryResult{
		Data: make(map[string]interface{}),
		Graph: Graph{
			Nodes: []Node{},
			Edges: []Edge{},
		},
	}

	// Execute query for each context
	for _, context := range ast.Contexts {
		executor, err := GetContextQueryExecutor(context)
		if err != nil {
			return combinedResults, fmt.Errorf("error getting executor for context %s: %v", context, err)
		}

		// Create a modified AST with prefixed variables
		modifiedAst := prefixVariables(ast, context)

		// Use ExecuteSingleQuery instead of Execute
		result, err := executor.ExecuteSingleQuery(modifiedAst, namespace)
		if err != nil {
			return combinedResults, fmt.Errorf("error executing query in context %s: %v", context, err)
		}

		// Merge results
		for k, v := range result.Data {
			combinedResults.Data[k] = v
		}
		combinedResults.Graph.Nodes = append(combinedResults.Graph.Nodes, result.Graph.Nodes...)
		combinedResults.Graph.Edges = append(combinedResults.Graph.Edges, result.Graph.Edges...)
	}

	return combinedResults, nil
}

// Helper function to prefix variables in the AST
func prefixVariables(ast *Expression, context string) *Expression {
	modified := &Expression{
		Clauses:  make([]Clause, len(ast.Clauses)),
		Contexts: ast.Contexts,
	}

	for i, clause := range ast.Clauses {
		switch c := clause.(type) {
		case *MatchClause:
			modified.Clauses[i] = prefixMatchClause(c, context)
		case *ReturnClause:
			modified.Clauses[i] = prefixReturnClause(c, context)
		case *SetClause:
			modified.Clauses[i] = prefixSetClause(c, context)
		case *DeleteClause:
			modified.Clauses[i] = prefixDeleteClause(c, context)
		case *CreateClause:
			modified.Clauses[i] = prefixCreateClause(c, context)
		}
	}

	return modified
}

// Helper functions to prefix variables in each clause type
func prefixMatchClause(c *MatchClause, context string) *MatchClause {
	modified := &MatchClause{
		Nodes:         make([]*NodePattern, len(c.Nodes)),
		Relationships: make([]*Relationship, len(c.Relationships)),
		ExtraFilters:  make([]*Filter, len(c.ExtraFilters)),
	}

	// Prefix node names
	for i, node := range c.Nodes {
		modified.Nodes[i] = &NodePattern{
			ResourceProperties: &ResourceProperties{
				Name:       context + "_" + node.ResourceProperties.Name,
				Kind:       node.ResourceProperties.Kind,
				Properties: node.ResourceProperties.Properties,
				JsonData:   node.ResourceProperties.JsonData,
			},
		}
	}

	// Prefix relationships
	for i, rel := range c.Relationships {
		modified.Relationships[i] = &Relationship{
			ResourceProperties: rel.ResourceProperties,
			Direction:          rel.Direction,
			LeftNode: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name:       context + "_" + rel.LeftNode.ResourceProperties.Name,
					Kind:       rel.LeftNode.ResourceProperties.Kind,
					Properties: rel.LeftNode.ResourceProperties.Properties,
					JsonData:   rel.LeftNode.ResourceProperties.JsonData,
				},
			},
			RightNode: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name:       context + "_" + rel.RightNode.ResourceProperties.Name,
					Kind:       rel.RightNode.ResourceProperties.Kind,
					Properties: rel.RightNode.ResourceProperties.Properties,
					JsonData:   rel.RightNode.ResourceProperties.JsonData,
				},
			},
		}
	}

	// Prefix filter variables
	for i, extraFilter := range c.ExtraFilters {
		if extraFilter.Type == "KeyValuePair" {
			filter := extraFilter.KeyValuePair
			parts := strings.Split(filter.Key, ".")
			if len(parts) > 0 {
				parts[0] = context + "_" + parts[0]
			}
			modified.ExtraFilters[i] = &Filter{
				Type: "KeyValuePair",
				KeyValuePair: &KeyValuePair{
					Key:      strings.Join(parts, "."),
					Value:    filter.Value,
					Operator: filter.Operator,
				},
			}
		}
	}

	return modified
}

func prefixReturnClause(c *ReturnClause, context string) *ReturnClause {
	modified := &ReturnClause{
		Items: make([]*ReturnItem, len(c.Items)),
	}

	for i, item := range c.Items {
		// Split the JsonPath to prefix the variable name
		parts := strings.Split(item.JsonPath, ".")
		if len(parts) > 0 {
			parts[0] = context + "_" + parts[0]
		}

		modified.Items[i] = &ReturnItem{
			JsonPath:  strings.Join(parts, "."),
			Alias:     item.Alias,
			Aggregate: item.Aggregate,
		}
	}

	return modified
}

func prefixSetClause(c *SetClause, context string) *SetClause {
	modified := &SetClause{
		KeyValuePairs: make([]*KeyValuePair, len(c.KeyValuePairs)),
	}

	for i, kvp := range c.KeyValuePairs {
		// Split the key to prefix the variable name
		parts := strings.Split(kvp.Key, ".")
		if len(parts) > 0 {
			parts[0] = context + "_" + parts[0]
		}

		modified.KeyValuePairs[i] = &KeyValuePair{
			Key:      strings.Join(parts, "."),
			Value:    kvp.Value,
			Operator: kvp.Operator,
		}
	}

	return modified
}

func prefixDeleteClause(c *DeleteClause, context string) *DeleteClause {
	modified := &DeleteClause{
		NodeIds: make([]string, len(c.NodeIds)),
	}

	for i, nodeId := range c.NodeIds {
		modified.NodeIds[i] = context + "_" + nodeId
	}

	return modified
}

func prefixCreateClause(c *CreateClause, context string) *CreateClause {
	modified := &CreateClause{
		Nodes:         make([]*NodePattern, len(c.Nodes)),
		Relationships: make([]*Relationship, len(c.Relationships)),
	}

	// Prefix node names
	for i, node := range c.Nodes {
		modified.Nodes[i] = &NodePattern{
			ResourceProperties: &ResourceProperties{
				Name:       context + "_" + node.ResourceProperties.Name,
				Kind:       node.ResourceProperties.Kind,
				Properties: node.ResourceProperties.Properties,
				JsonData:   node.ResourceProperties.JsonData,
			},
		}
	}

	// Prefix relationship node references
	for i, rel := range c.Relationships {
		modified.Relationships[i] = &Relationship{
			ResourceProperties: rel.ResourceProperties,
			Direction:          rel.Direction,
			LeftNode: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name:       context + "_" + rel.LeftNode.ResourceProperties.Name,
					Kind:       rel.LeftNode.ResourceProperties.Kind,
					Properties: rel.LeftNode.ResourceProperties.Properties,
					JsonData:   rel.LeftNode.ResourceProperties.JsonData,
				},
			},
			RightNode: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name:       context + "_" + rel.RightNode.ResourceProperties.Name,
					Kind:       rel.RightNode.ResourceProperties.Kind,
					Properties: rel.RightNode.ResourceProperties.Properties,
					JsonData:   rel.RightNode.ResourceProperties.JsonData,
				},
			},
		}
	}

	return modified
}
