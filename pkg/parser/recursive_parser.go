package parser

import (
	"fmt"
	"strings"
)

type Parser struct {
	lexer   *Lexer
	current Token
}

func NewRecursiveParser(input string) *Parser {
	lexer := NewLexer(input)
	return &Parser{
		lexer: lexer,
	}
}

// Parse is the entry point for parsing a Cyphernetes query
func (p *Parser) Parse() (*Expression, error) {
	p.advance() // Get first token
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.current.Type != EOF {
		return nil, fmt.Errorf("unexpected token after expression: %v", p.current)
	}

	return expr, nil
}

// parseExpression parses: Expression -> IN Contexts? (MatchClause | CreateClause) (SetClause | DeleteClause | ReturnClause)?
func (p *Parser) parseExpression() (*Expression, error) {
	var contexts []string
	var clauses []Clause

	// Check for IN clause
	if p.current.Type == IN {
		p.advance()
		var err error
		contexts, err = p.parseContexts()
		if err != nil {
			return nil, fmt.Errorf("parsing contexts: %w", err)
		}
	}

	// Parse first mandatory clause (Match or Create)
	firstClause, err := p.parseFirstClause()
	if err != nil {
		return nil, fmt.Errorf("parsing first clause: %w", err)
	}
	clauses = append(clauses, firstClause)

	// Parse optional second clause
	if secondClause, err := p.parseSecondClause(); err != nil {
		return nil, fmt.Errorf("parsing second clause: %w", err)
	} else if secondClause != nil {
		clauses = append(clauses, secondClause)
	}

	return &Expression{
		Contexts: contexts,
		Clauses:  clauses,
	}, nil
}

// parseFirstClause parses either a MATCH or CREATE clause
func (p *Parser) parseFirstClause() (Clause, error) {
	switch p.current.Type {
	case MATCH:
		return p.parseMatchClause()
	case CREATE:
		return p.parseCreateClause()
	default:
		return nil, fmt.Errorf("expected MATCH or CREATE, got %v", p.current)
	}
}

// parseMatchClause parses: MATCH NodeRelationshipList (WHERE KeyValuePairs)?
func (p *Parser) parseMatchClause() (*MatchClause, error) {
	if p.current.Type != MATCH {
		return nil, fmt.Errorf("expected MATCH, got %v", p.current)
	}
	p.advance()

	nodeRels, err := p.parseNodeRelationshipList()
	if err != nil {
		return nil, err
	}

	var filters []*KeyValuePair
	if p.current.Type == WHERE {
		p.advance()
		filters, err = p.parseKeyValuePairs()
		if err != nil {
			return nil, err
		}
	}

	return &MatchClause{
		Nodes:         nodeRels.Nodes,
		Relationships: nodeRels.Relationships,
		ExtraFilters:  filters,
	}, nil
}

// parseCreateClause parses: CREATE NodeRelationshipList
func (p *Parser) parseCreateClause() (*CreateClause, error) {
	if p.current.Type != CREATE {
		return nil, fmt.Errorf("expected CREATE, got %v", p.current)
	}
	p.advance()

	nodeRels, err := p.parseNodeRelationshipList()
	if err != nil {
		return nil, err
	}

	return &CreateClause{
		Nodes:         nodeRels.Nodes,
		Relationships: nodeRels.Relationships,
	}, nil
}

// parseNodeRelationshipList parses node patterns and relationships
func (p *Parser) parseNodeRelationshipList() (*NodeRelationshipList, error) {
	var nodes []*NodePattern
	var relationships []*Relationship

	// Parse first node
	node, err := p.parseNodePattern()
	if err != nil {
		return nil, err
	}
	nodes = append(nodes, node)

	// Parse subsequent relationships and nodes
	for {
		if p.current.Type == COMMA {
			p.advance()
			node, err := p.parseNodePattern()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
			continue
		}

		if isRelationshipStart(p.current.Type) {
			rel, rightNode, err := p.parseRelationshipAndNode()
			if err != nil {
				return nil, err
			}
			rel.LeftNode = nodes[len(nodes)-1]
			rel.RightNode = rightNode
			relationships = append(relationships, rel)
			nodes = append(nodes, rightNode)
			continue
		}

		break
	}

	return &NodeRelationshipList{
		Nodes:         nodes,
		Relationships: relationships,
	}, nil
}

// parseNodePattern parses: LPAREN ResourceProperties RPAREN | LPAREN IDENT RPAREN
func (p *Parser) parseNodePattern() (*NodePattern, error) {
	if p.current.Type != LPAREN {
		return nil, fmt.Errorf("expected (, got %v", p.current)
	}
	p.advance()

	var resourceProps *ResourceProperties
	if p.current.Type == IDENT {
		name := p.current.Literal
		p.advance()

		if p.current.Type == COLON {
			p.advance()
			resourceProps, err := p.parseResourceProperties(name)
			if err != nil {
				return nil, err
			}
			if p.current.Type != RPAREN {
				return nil, fmt.Errorf("expected ), got %v", p.current)
			}
			p.advance()
			return &NodePattern{ResourceProperties: resourceProps}, nil
		}

		if p.current.Type != RPAREN {
			return nil, fmt.Errorf("expected ), got %v", p.current)
		}
		p.advance()
		return &NodePattern{&ResourceProperties{Name: name}}, nil
	}

	return nil, fmt.Errorf("expected identifier, got %v", p.current)
}

// parseResourceProperties parses the properties of a node or relationship
func (p *Parser) parseResourceProperties(name string) (*ResourceProperties, error) {
	if p.current.Type != IDENT {
		return nil, fmt.Errorf("expected kind identifier, got %v", p.current)
	}
	kind := p.current.Literal
	p.advance()

	var properties *Properties
	var jsonData string

	if p.current.Type == LBRACE {
		p.advance()
		if p.current.Type == JSONDATA {
			jsonData = p.current.Literal
			p.advance()
		} else {
			props, err := p.parseProperties()
			if err != nil {
				return nil, err
			}
			properties = props
		}
		if p.current.Type != RBRACE {
			return nil, fmt.Errorf("expected }, got %v", p.current)
		}
		p.advance()
	}

	return &ResourceProperties{
		Name:       name,
		Kind:       kind,
		Properties: properties,
		JsonData:   jsonData,
	}, nil
}

// Helper function to check if a token starts a relationship
func isRelationshipStart(t TokenType) bool {
	switch t {
	case REL_NOPROPS_RIGHT, REL_NOPROPS_LEFT, REL_NOPROPS_BOTH, REL_NOPROPS_NONE,
		REL_BEGINPROPS_LEFT, REL_BEGINPROPS_NONE:
		return true
	default:
		return false
	}
}

// Helper method to advance the lexer
func (p *Parser) advance() {
	p.current = p.lexer.NextToken()
}

// parseRelationshipAndNode parses a relationship token followed by a node pattern
func (p *Parser) parseRelationshipAndNode() (*Relationship, *NodePattern, error) {
	var direction Direction
	var resourceProps *ResourceProperties

	// Determine relationship direction and properties based on token type
	switch p.current.Type {
	case REL_NOPROPS_RIGHT:
		direction = Right
	case REL_NOPROPS_LEFT:
		direction = Left
	case REL_NOPROPS_BOTH:
		direction = Both
	case REL_NOPROPS_NONE:
		direction = None
	case REL_BEGINPROPS_LEFT:
		direction = Left
		p.advance()
		var err error
		resourceProps, err = p.parseRelationshipProperties()
		if err != nil {
			return nil, nil, err
		}
		if p.current.Type != REL_ENDPROPS_NONE {
			return nil, nil, fmt.Errorf("expected relationship end token, got %v", p.current)
		}
	case REL_BEGINPROPS_NONE:
		direction = None
		p.advance()
		var err error
		resourceProps, err = p.parseRelationshipProperties()
		if err != nil {
			return nil, nil, err
		}
		if p.current.Type != REL_ENDPROPS_RIGHT {
			return nil, nil, fmt.Errorf("expected relationship end token, got %v", p.current)
		}
	default:
		return nil, nil, fmt.Errorf("unexpected relationship token: %v", p.current)
	}
	p.advance()

	// Parse the right node
	rightNode, err := p.parseNodePattern()
	if err != nil {
		return nil, nil, err
	}

	rel := &Relationship{
		ResourceProperties: resourceProps,
		Direction:          direction,
	}

	return rel, rightNode, nil
}

// parseSecondClause parses optional SET, DELETE, or RETURN clause
func (p *Parser) parseSecondClause() (Clause, error) {
	switch p.current.Type {
	case SET:
		return p.parseSetClause()
	case DELETE:
		return p.parseDeleteClause()
	case RETURN:
		return p.parseReturnClause()
	case EOF:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected token for second clause: %v", p.current)
	}
}

// parseSetClause parses: SET KeyValuePairs
func (p *Parser) parseSetClause() (*SetClause, error) {
	if p.current.Type != SET {
		return nil, fmt.Errorf("expected SET, got %v", p.current)
	}
	p.advance()

	pairs, err := p.parseKeyValuePairs()
	if err != nil {
		return nil, err
	}

	return &SetClause{KeyValuePairs: pairs}, nil
}

// parseDeleteClause parses: DELETE NodeIds
func (p *Parser) parseDeleteClause() (*DeleteClause, error) {
	if p.current.Type != DELETE {
		return nil, fmt.Errorf("expected DELETE, got %v", p.current)
	}
	p.advance()

	var nodeIds []string
	for {
		if p.current.Type != IDENT {
			return nil, fmt.Errorf("expected identifier, got %v", p.current)
		}
		nodeIds = append(nodeIds, p.current.Literal)
		p.advance()

		if p.current.Type != COMMA {
			break
		}
		p.advance()
	}

	return &DeleteClause{NodeIds: nodeIds}, nil
}

// parseReturnClause parses: RETURN ReturnItems
func (p *Parser) parseReturnClause() (*ReturnClause, error) {
	if p.current.Type != RETURN {
		return nil, fmt.Errorf("expected RETURN, got %v", p.current)
	}
	p.advance()

	items, err := p.parseReturnItems()
	if err != nil {
		return nil, err
	}

	return &ReturnClause{Items: items}, nil
}

// parseReturnItems parses a list of return items
func (p *Parser) parseReturnItems() ([]*ReturnItem, error) {
	var items []*ReturnItem

	for {
		item, err := p.parseReturnItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)

		if p.current.Type != COMMA {
			break
		}
		p.advance()
	}

	return items, nil
}

// parseReturnItem parses a single return item with optional aggregation and alias
func (p *Parser) parseReturnItem() (*ReturnItem, error) {
	var item ReturnItem

	// Check for aggregation functions
	if p.current.Type == COUNT || p.current.Type == SUM {
		item.Aggregate = strings.ToUpper(p.current.Literal)
		p.advance()

		if p.current.Type != LBRACE {
			return nil, fmt.Errorf("expected {, got %v", p.current)
		}
		p.advance()
	}

	// Parse JsonPath
	if p.current.Type != JSONPATH {
		return nil, fmt.Errorf("expected JSONPATH, got %v", p.current)
	}
	item.JsonPath = p.current.Literal
	p.advance()

	// Handle closing brace for aggregation
	if item.Aggregate != "" {
		if p.current.Type != RBRACE {
			return nil, fmt.Errorf("expected }, got %v", p.current)
		}
		p.advance()
	}

	// Check for AS alias
	if p.current.Type == AS {
		p.advance()
		if p.current.Type != IDENT {
			return nil, fmt.Errorf("expected identifier after AS, got %v", p.current)
		}
		item.Alias = p.current.Literal
		p.advance()
	}

	return &item, nil
}

// parseContexts parses a list of context identifiers
func (p *Parser) parseContexts() ([]string, error) {
	var contexts []string

	for {
		if p.current.Type != IDENT {
			return nil, fmt.Errorf("expected identifier, got %v", p.current)
		}
		contexts = append(contexts, p.current.Literal)
		p.advance()

		if p.current.Type != COMMA {
			break
		}
		p.advance()
	}

	return contexts, nil
}

// parseProperties parses a list of key-value properties
func (p *Parser) parseProperties() (*Properties, error) {
	var propertyList []*Property

	for {
		if p.current.Type != JSONPATH {
			return nil, fmt.Errorf("expected JSONPATH, got %v", p.current)
		}
		key := p.current.Literal
		p.advance()

		if p.current.Type != COLON {
			return nil, fmt.Errorf("expected :, got %v", p.current)
		}
		p.advance()

		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		propertyList = append(propertyList, &Property{
			Key:   key,
			Value: value,
		})

		if p.current.Type != COMMA {
			break
		}
		p.advance()
	}

	return &Properties{PropertyList: propertyList}, nil
}

// parseKeyValuePairs parses a list of key-value pairs with operators
func (p *Parser) parseKeyValuePairs() ([]*KeyValuePair, error) {
	var pairs []*KeyValuePair

	for {
		if p.current.Type != JSONPATH {
			return nil, fmt.Errorf("expected JSONPATH, got %v", p.current)
		}
		key := p.current.Literal
		p.advance()

		operator, err := p.parseOperator()
		if err != nil {
			return nil, err
		}

		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		pairs = append(pairs, &KeyValuePair{
			Key:      key,
			Value:    value,
			Operator: operator,
		})

		if p.current.Type != COMMA {
			break
		}
		p.advance()
	}

	return pairs, nil
}

// parseOperator parses comparison operators
func (p *Parser) parseOperator() (string, error) {
	switch p.current.Type {
	case EQUALS:
		p.advance()
		return "EQUALS", nil
	case NOT_EQUALS:
		p.advance()
		return "NOT_EQUALS", nil
	case GREATER_THAN:
		p.advance()
		return "GREATER_THAN", nil
	case LESS_THAN:
		p.advance()
		return "LESS_THAN", nil
	case GREATER_THAN_EQUALS:
		p.advance()
		return "GREATER_THAN_EQUALS", nil
	case LESS_THAN_EQUALS:
		p.advance()
		return "LESS_THAN_EQUALS", nil
	case CONTAINS:
		p.advance()
		return "CONTAINS", nil
	case REGEX_COMPARE:
		p.advance()
		return "REGEX_COMPARE", nil
	default:
		return "", fmt.Errorf("expected operator, got %v", p.current)
	}
}

// parseValue parses literal values (string, int, boolean, jsondata, or null)
func (p *Parser) parseValue() (interface{}, error) {
	switch p.current.Type {
	case STRING:
		value := strings.Trim(p.current.Literal, "\"")
		p.advance()
		return value, nil
	case INT:
		value := p.current.Literal
		p.advance()
		return value, nil
	case BOOLEAN:
		value := strings.ToUpper(p.current.Literal) == "TRUE"
		p.advance()
		return value, nil
	case JSONDATA:
		value := p.current.Literal
		p.advance()
		return value, nil
	case NULL:
		p.advance()
		return nil, nil
	default:
		return nil, fmt.Errorf("expected value, got %v", p.current)
	}
}
