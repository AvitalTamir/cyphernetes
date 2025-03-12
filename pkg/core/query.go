package core

import (
	"fmt"
	"strings"
)

func (q *QueryExecutor) rewriteQueryForKindlessNodes(expr *Expression) (*Expression, error) {
	// Find all kindless nodes and their relationships
	var kindlessNodes []*NodePattern
	var relationships []*Relationship

	for _, c := range expr.Clauses {
		if matchClause, ok := c.(*MatchClause); ok {
			// Find kindless nodes
			for _, node := range matchClause.Nodes {
				if node.ResourceProperties.Kind == "" {
					kindlessNodes = append(kindlessNodes, node)
				}
			}

			// Collect relationships
			relationships = append(relationships, matchClause.Relationships...)

			// Check for standalone kindless nodes
			for _, node := range kindlessNodes {
				isInRelationship := false
				for _, rel := range matchClause.Relationships {
					if rel.LeftNode.ResourceProperties.Name == node.ResourceProperties.Name ||
						rel.RightNode.ResourceProperties.Name == node.ResourceProperties.Name {
						isInRelationship = true
						break
					}
				}
				if !isInRelationship {
					return nil, fmt.Errorf("kindless nodes may only be used in a relationship")
				}
			}

			// Check for kindless-to-kindless chains in relationships
			for _, rel := range matchClause.Relationships {
				if rel.LeftNode.ResourceProperties.Kind == "" && rel.RightNode.ResourceProperties.Kind == "" {
					return nil, fmt.Errorf("chaining two unknown nodes (kindless-to-kindless) is not supported - at least one node in a relationship must have a known kind")
				}
			}
		}
	}

	// If no kindless nodes, no rewrite needed
	if len(kindlessNodes) == 0 {
		return nil, nil
	}

	// Find potential kinds for each kindless node
	var potentialKinds []string
	var err error
	if mockFindPotentialKinds != nil {
		// Use mock function in tests
		potentialKinds = mockFindPotentialKinds(relationships)
	} else {
		// Use real function in production
		potentialKinds, err = FindPotentialKindsIntersection(relationships, q.provider)
		if err != nil {
			return nil, fmt.Errorf("unable to determine kind for nodes in relationship >> %s", err)
		}
	}

	if len(potentialKinds) == 0 {
		return nil, fmt.Errorf("unable to determine kind for nodes in relationship")
	}

	// Build expanded query
	var matchParts []string
	var returnParts []string
	var setParts []string
	var deleteParts []string
	var whereParts []string
	var seenNodes = make(map[string]bool)

	// For each potential kind, create a match pattern and return item
	for i, kind := range potentialKinds {
		for _, clause := range expr.Clauses {
			switch c := clause.(type) {
			case *MatchClause:
				// Build match pattern
				var nodeParts []string
				for _, rel := range c.Relationships {
					var leftNodeStr, rightNodeStr string

					// Build left node pattern
					if rel.LeftNode.ResourceProperties.Kind == "" {
						leftNodeStr = fmt.Sprintf("(%s__exp__%d:%s", rel.LeftNode.ResourceProperties.Name, i, kind)
					} else {
						leftNodeStr = fmt.Sprintf("(%s__exp__%d:%s", rel.LeftNode.ResourceProperties.Name, i, rel.LeftNode.ResourceProperties.Kind)
					}

					// Add properties to left node if any
					if rel.LeftNode.ResourceProperties.Properties != nil {
						props := make([]string, 0)
						for _, prop := range rel.LeftNode.ResourceProperties.Properties.PropertyList {
							var valueStr string
							switch v := prop.Value.(type) {
							case string:
								valueStr = fmt.Sprintf("\"%s\"", v)
							default:
								valueStr = fmt.Sprintf("%v", v)
							}
							props = append(props, fmt.Sprintf("%s: %s", prop.Key, valueStr))
						}
						leftNodeStr += fmt.Sprintf(" {%s}", strings.Join(props, ", "))
					}
					leftNodeStr += ")"

					// Build right node pattern
					if rel.RightNode.ResourceProperties.Kind == "" {
						rightNodeStr = fmt.Sprintf("(%s__exp__%d:%s", rel.RightNode.ResourceProperties.Name, i, kind)
					} else {
						rightNodeStr = fmt.Sprintf("(%s__exp__%d:%s", rel.RightNode.ResourceProperties.Name, i, rel.RightNode.ResourceProperties.Kind)
					}

					// Add properties to right node if any
					if rel.RightNode.ResourceProperties.Properties != nil {
						props := make([]string, 0)
						for _, prop := range rel.RightNode.ResourceProperties.Properties.PropertyList {
							var valueStr string
							switch v := prop.Value.(type) {
							case string:
								valueStr = fmt.Sprintf("\"%s\"", v)
							default:
								valueStr = fmt.Sprintf("%v", v)
							}
							props = append(props, fmt.Sprintf("%s: %s", prop.Key, valueStr))
						}
						rightNodeStr += fmt.Sprintf(" {%s}", strings.Join(props, ", "))
					}
					rightNodeStr += ")"

					// Add relationship pattern
					nodeParts = append(nodeParts, fmt.Sprintf("%s->%s", leftNodeStr, rightNodeStr))
				}
				if len(nodeParts) > 0 {
					matchParts = append(matchParts, strings.Join(nodeParts, ", "))
				}

				// Handle WHERE conditions from ExtraFilters
				for _, extraFilter := range c.ExtraFilters {
					if extraFilter.Type == "KeyValuePair" {
						filter := extraFilter.KeyValuePair
						parts := strings.Split(filter.Key, ".")
						if len(parts) > 0 {
							nodeName := parts[0]
							propertyPath := strings.Join(parts[1:], ".")

							for j := 0; j < len(potentialKinds); j++ {
								varName := fmt.Sprintf("%s__exp__%d", nodeName, j)
								var valueStr string
								switch v := filter.Value.(type) {
								case string:
									valueStr = fmt.Sprintf("\"%s\"", v)
								default:
									valueStr = fmt.Sprintf("%v", v)
								}
								// Map operator names to symbols
								operator := filter.Operator
								switch operator {
								case "EQUALS":
									operator = "="
								case "NOT_EQUALS":
									operator = "!="
								case "GREATER_THAN":
									operator = ">"
								case "LESS_THAN":
									operator = "<"
								case "GREATER_THAN_EQUALS":
									operator = ">="
								case "LESS_THAN_EQUALS":
									operator = "<="
								case "REGEX_COMPARE":
									operator = "=~"
								case "CONTAINS":
									operator = "CONTAINS"
								case "":
									operator = "="
								}

								notPrefix := ""
								if filter.IsNegated {
									notPrefix = "NOT "
								}
								whereParts = append(whereParts, fmt.Sprintf("%s%s.%s %s %s", notPrefix, varName, propertyPath, operator, valueStr))
							}
						}
					}
				}

			case *ReturnClause:
				// Build return items
				for _, item := range c.Items {
					if !seenNodes[item.JsonPath] {
						seenNodes[item.JsonPath] = true
						// Add a return item for each iteration
						for j := 0; j < len(potentialKinds); j++ {
							var returnItem string
							if strings.Contains(item.JsonPath, ".") {
								parts := strings.SplitN(item.JsonPath, ".", 2)
								varName := fmt.Sprintf("%s__exp__%d", parts[0], j)
								returnPath := fmt.Sprintf("%s.%s", varName, parts[1])
								if item.Aggregate != "" {
									returnItem = fmt.Sprintf("%s {%s}", item.Aggregate, returnPath)
								} else {
									returnItem = returnPath
								}
							} else {
								varName := fmt.Sprintf("%s__exp__%d", item.JsonPath, j)
								if item.Aggregate != "" {
									returnItem = fmt.Sprintf("%s {%s}", item.Aggregate, varName)
								} else {
									returnItem = varName
								}
							}
							// Add AS alias with expansion pattern
							if item.Aggregate != "" {
								aggType := strings.ToLower(item.Aggregate)
								if item.Alias != "" {
									returnItem = fmt.Sprintf("%s AS __exp__%s__%s__%d", returnItem, aggType, item.Alias, j)
								} else {
									// For non-aliased aggregations, use the format <aggregate_type>_<node>_<path>
									aliasPath := item.JsonPath
									aliasPath = strings.Replace(aliasPath, ".", "_", -1)
									returnItem = fmt.Sprintf("%s AS __exp__%s__%s_%s__%d", returnItem, aggType, aggType, aliasPath, j)
								}
							} else if item.Alias != "" {
								returnItem = fmt.Sprintf("%s AS %s", returnItem, item.Alias)
							}
							returnParts = append(returnParts, returnItem)
						}
					}
				}
			case *SetClause:
				// Build set items
				for _, kvp := range c.KeyValuePairs {
					if strings.Contains(kvp.Key, ".") {
						parts := strings.SplitN(kvp.Key, ".", 2)
						// If the node being set is kindless, we need to create a set clause for each potential kind
						if isKindless(parts[0], kindlessNodes) {
							for j := 0; j < len(potentialKinds); j++ {
								varName := fmt.Sprintf("%s__exp__%d", parts[0], j)
								setPath := fmt.Sprintf("%s.%s", varName, parts[1])
								var valueStr string
								switch v := kvp.Value.(type) {
								case string:
									valueStr = fmt.Sprintf("\"%s\"", v)
								default:
									valueStr = fmt.Sprintf("%v", v)
								}
								setParts = append(setParts, fmt.Sprintf("%s = %s", setPath, valueStr))
							}
						} else {
							// If the node is not kindless, just use it as is
							varName := fmt.Sprintf("%s__exp__0", parts[0])
							setPath := fmt.Sprintf("%s.%s", varName, parts[1])
							var valueStr string
							switch v := kvp.Value.(type) {
							case string:
								valueStr = fmt.Sprintf("'%s'", v)
							default:
								valueStr = fmt.Sprintf("%v", v)
							}
							setParts = append(setParts, fmt.Sprintf("%s = %s", setPath, valueStr))
						}
					}
				}
			case *DeleteClause:
				// Build delete items
				for _, nodeId := range c.NodeIds {
					// If the node being deleted is kindless, we need to create a delete clause for each potential kind
					if isKindless(nodeId, kindlessNodes) {
						for j := 0; j < len(potentialKinds); j++ {
							varName := fmt.Sprintf("%s__exp__%d", nodeId, j)
							deleteParts = append(deleteParts, varName)
						}
					} else {
						// If the node is not kindless, just use it as is
						varName := fmt.Sprintf("%s__exp__0", nodeId)
						deleteParts = append(deleteParts, varName)
					}
				}
			}
		}
	}

	// Combine into final query
	var queryParts []string
	if len(matchParts) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("MATCH %s", strings.Join(matchParts, ", ")))
	}
	if len(whereParts) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("WHERE %s", strings.Join(whereParts, ", ")))
	}
	if len(setParts) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("SET %s", strings.Join(setParts, ", ")))
	}
	if len(deleteParts) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("DELETE %s", strings.Join(deleteParts, ", ")))
	}
	if len(returnParts) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("RETURN %s", strings.Join(returnParts, ", ")))
	}

	query := strings.Join(queryParts, " ")

	// Log the expanded query for debugging
	debugLog(fmt.Sprintf("Expanded query: %s\n", query))

	// Parse the expanded query into a new AST
	newAst, err := ParseQuery(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing expanded query: %w", err)
	}

	return newAst, nil
}

// QueryExpandedError is a special error type that indicates the query was expanded
type QueryExpandedError struct {
	ExpandedQuery string
}

func (e *QueryExpandedError) Error() string {
	return fmt.Sprintf("query expanded to: %s", e.ExpandedQuery)
}

func isKindless(nodeName string, kindlessNodes []*NodePattern) bool {
	for _, node := range kindlessNodes {
		if node.ResourceProperties.Name == nodeName {
			return true
		}
	}
	return false
}
