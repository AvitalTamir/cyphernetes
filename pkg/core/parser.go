package core

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

type Parser struct {
	lexer            *Lexer
	current          Token
	pos              int
	inCreate         bool
	anonymousCounter int
	matchVariables   map[string]*NodePattern // Track variables defined in MATCH clause
	matchNodes       []*NodePattern          // Track nodes from the current match clause
}

func NewParser(input string) *Parser {
	lexer := NewLexer(input)
	return &Parser{
		lexer:            lexer,
		anonymousCounter: 0,
		matchVariables:   make(map[string]*NodePattern),
		matchNodes:       make([]*NodePattern, 0),
	}
}

// Parse is the entry point for parsing a Cyphernetes query
func (p *Parser) Parse() (*Expression, error) {
	p.advance() // Get first token
	debugLog("Starting parse with token: %v", p.current)

	var contexts []string
	var clauses []Clause

	// Check for IN clause
	if p.current.Type == IN {
		p.advance()
		p.lexer.SetParsingContexts(true)
		var err error
		contexts, err = p.parseContexts()
		if err != nil {
			return nil, fmt.Errorf("parsing contexts: %w", err)
		}
		p.lexer.SetParsingContexts(false)
	}

	// Parse first clause (must be MATCH or CREATE)
	if p.current.Type != MATCH && p.current.Type != CREATE {
		return nil, fmt.Errorf("expected MATCH or CREATE, got \"%v\"", p.current.Literal)
	}

	firstClause, err := p.parseFirstClause()
	if err != nil {
		if strings.Contains(err.Error(), "unexpected relationship token") {
			return nil, err
		}
		return nil, fmt.Errorf("parsing first clause: %w", err)
	}
	clauses = append(clauses, firstClause)

	// Handle WHERE clause if present
	if p.current.Type == WHERE {
		if matchClause, ok := firstClause.(*MatchClause); ok {
			p.advance()
			filters, err := p.parseFilters()
			if err != nil {
				return nil, fmt.Errorf("parsing WHERE clause: %w", err)
			}
			matchClause.ExtraFilters = filters
		} else {
			return nil, fmt.Errorf("WHERE clause can only follow MATCH")
		}
	}

	// Parse optional second and third clauses according to valid combinations
	switch p.current.Type {
	case SET:
		setClause, err := p.parseSetClause()
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, setClause)

		// After SET, only RETURN is valid
		if p.current.Type == RETURN {
			returnClause, err := p.parseReturnClause()
			if err != nil {
				return nil, err
			}
			clauses = append(clauses, returnClause)
		}

	case DELETE:
		if _, ok := firstClause.(*MatchClause); !ok {
			return nil, fmt.Errorf("DELETE can only follow MATCH")
		}
		deleteClause, err := p.parseDeleteClause()
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, deleteClause)

	case CREATE:
		if _, ok := firstClause.(*MatchClause); !ok {
			return nil, fmt.Errorf("CREATE can only follow MATCH in this position")
		}
		createClause, err := p.parseCreateClause()
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, createClause)

		// After CREATE, only RETURN is valid
		if p.current.Type == RETURN {
			returnClause, err := p.parseReturnClause()
			if err != nil {
				return nil, err
			}
			clauses = append(clauses, returnClause)
		}

	case RETURN:
		returnClause, err := p.parseReturnClause()
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, returnClause)
	}

	// Check for invalid tokens first
	if p.current.Type == '<' {
		debugLog("Found invalid token '<' before EOF")
		return nil, fmt.Errorf("unexpected relationship token: \"%v\"", p.current.Literal)
	}

	// Then check for EOF
	debugLog("Checking for EOF, current token: %v", p.current)
	if p.current.Type != EOF {
		if p.current.Type == ILLEGAL && strings.HasPrefix(p.current.Literal, "<") {
			return nil, fmt.Errorf("unexpected relationship token: \"%v\"", p.current.Literal)
		}
		return nil, fmt.Errorf("unexpected token after expression: \"%v\"", p.current.Literal)
	}

	// Check for incomplete expression last
	if len(clauses) < 2 && !isCreateClause(clauses[0]) {
		return nil, fmt.Errorf("incomplete expression")
	}

	return &Expression{
		Contexts: contexts,
		Clauses:  clauses,
	}, nil
}

func isCreateClause(c Clause) bool {
	_, ok := c.(*CreateClause)
	return ok
}

// parseFirstClause parses either a MATCH or CREATE clause
func (p *Parser) parseFirstClause() (Clause, error) {
	switch p.current.Type {
	case CREATE:
		p.inCreate = true                     // Set flag
		defer func() { p.inCreate = false }() // Reset when done
		return p.parseCreateClause()
	case MATCH:
		return p.parseMatchClause()
	default:
		return nil, fmt.Errorf("expected MATCH or CREATE, got \"%v\"", p.current.Literal)
	}
}

// parseMatchClause parses: MATCH NodeRelationshipList (WHERE KeyValuePairs)?
func (p *Parser) parseMatchClause() (*MatchClause, error) {
	if p.current.Type != MATCH {
		return nil, fmt.Errorf("expected MATCH, got \"%v\"", p.current.Literal)
	}
	p.advance()

	nodeRels, err := p.parseNodeRelationshipList()
	if err != nil {
		return nil, err
	}

	// Store nodes for later use in WHERE clause validation
	p.matchNodes = nodeRels.Nodes

	// Track variables from the MATCH clause
	p.trackMatchVariables(nodeRels.Nodes)

	// Validate that we don't have standalone anonymous nodes
	if len(nodeRels.Nodes) == 1 && len(nodeRels.Relationships) == 0 {
		node := nodeRels.Nodes[0]
		if node.IsAnonymous || (node.ResourceProperties != nil && node.ResourceProperties.Name == "") {
			return nil, fmt.Errorf("standalone anonymous nodes are not allowed")
		}
	}

	var filters []*Filter
	if p.current.Type == WHERE {
		p.advance()
		filters, err = p.parseFilters()
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
	p.advance()                           // consume CREATE token
	p.inCreate = true                     // Set flag
	defer func() { p.inCreate = false }() // Reset when done

	nodeList, err := p.parseNodeRelationshipList()
	if err != nil {
		return nil, err
	}

	return &CreateClause{
		Nodes:         nodeList.Nodes,
		Relationships: nodeList.Relationships,
	}, nil
}

// parseNodeRelationshipList parses node patterns and relationships
func (p *Parser) parseNodeRelationshipList() (*NodeRelationshipList, error) {
	var nodes []*NodePattern
	var relationships []*Relationship

	debugLog("Parsing node relationship list, current token: %v", p.current.Literal)

	// Parse first node
	node, err := p.parseNodePattern()
	if err != nil {
		return nil, err
	}
	nodes = append(nodes, node)

	// Check for invalid relationship tokens before entering the loop
	if p.current.Type == '<' || (p.current.Type == '<' && p.lexer.Peek() == '<') {
		debugLog("Found invalid relationship token: \"%v\"", p.current.Literal)
		return nil, fmt.Errorf("unexpected relationship token: \"%v\"", p.current.Literal)
	}

	// Parse subsequent relationships and nodes
	for {
		debugLog("In relationship loop, current token: %v", p.current.Literal)

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

		if p.current.Type == COMMA {
			p.advance()
			node, err := p.parseNodePattern()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
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
	debugLog("Parsing node pattern, current token: \"%v\"", p.current.Literal)
	if p.current.Type != LPAREN {
		return nil, fmt.Errorf("expected (, got \"%v\"", p.current.Literal)
	}
	p.advance()

	var name string
	var resourceProps *ResourceProperties
	isAnonymous := false

	// Handle empty node ()
	if p.current.Type == RPAREN {
		p.advance()
		name = p.nextAnonymousVar()
		isAnonymous = true
		return &NodePattern{
			ResourceProperties: &ResourceProperties{
				Name: name,
				Kind: "",
			},
			IsAnonymous: isAnonymous,
		}, nil
	}

	// Handle variable name if present
	if p.current.Type == IDENT {
		name = p.current.Literal
		p.advance()
	} else {
		name = p.nextAnonymousVar()
		isAnonymous = true
	}

	// Handle kind if present
	if p.current.Type == COLON {
		p.advance()
		var err error
		resourceProps, err = p.parseResourceProperties(name)
		if err != nil {
			return nil, err
		}
	} else {
		// Initialize resourceProps for kindless node
		resourceProps = &ResourceProperties{
			Name: name,
			Kind: "", // Empty kind for variable-only nodes
		}

		// Check for properties
		if p.current.Type == LBRACE {
			p.advance()
			props, err := p.parseProperties()
			if err != nil {
				return nil, err
			}
			resourceProps.Properties = props
			if p.current.Type != RBRACE {
				return nil, fmt.Errorf("expected }, got \"%v\"", p.current.Literal)
			}
			p.advance()
		}
	}

	if p.current.Type != RPAREN {
		return nil, fmt.Errorf("expected ), got \"%v\"", p.current.Literal)
	}
	p.advance()

	// Check for invalid relationship tokens immediately after closing parenthesis
	debugLog("After node pattern, checking next token: \"%v\"", p.current.Literal)
	if p.current.Type == '<' {
		debugLog("Found invalid relationship token after node pattern")
		return nil, fmt.Errorf("unexpected relationship token: \"%v\"", p.current.Literal)
	}

	return &NodePattern{
		ResourceProperties: resourceProps,
		IsAnonymous:        isAnonymous,
	}, nil
}

// parseResourceProperties parses the properties of a node or relationship
func (p *Parser) parseResourceProperties(name string) (*ResourceProperties, error) {
	if p.current.Type != IDENT {
		return nil, fmt.Errorf("expected kind identifier, got \"%v\"", p.current.Literal)
	}
	kind := p.current.Literal
	p.advance()

	var properties *Properties
	var jsonData string

	if p.current.Type == LBRACE {
		p.advance()

		// Check if we're in a CREATE statement
		if p.inCreate && p.current.Type == IDENT {
			// Build the JSON object
			var jsonBuilder strings.Builder
			jsonBuilder.WriteString("{\n")
			depth := 1

			for depth > 0 {
				if p.current.Type == EOF {
					return nil, fmt.Errorf("unexpected EOF in JSON data")
				}

				// Format the JSON properly
				switch p.current.Type {
				case IDENT:
					jsonBuilder.WriteString("\"" + p.current.Literal + "\"")
				case STRING:
					jsonBuilder.WriteString(p.current.Literal)
				case COLON:
					jsonBuilder.WriteString(": ")
				case COMMA:
					jsonBuilder.WriteString(",\n")
				case LBRACE:
					jsonBuilder.WriteString("{\n")
					depth++
				case RBRACE:
					depth--
					if depth >= 0 {
						jsonBuilder.WriteString("\n}")
					}
				case LBRACKET:
					jsonBuilder.WriteString("[")
				case RBRACKET:
					jsonBuilder.WriteString("]")
				default:
					jsonBuilder.WriteString(p.current.Literal)
				}

				if depth > 0 {
					p.advance()
				}
			}
			jsonData = jsonBuilder.String()
			p.advance() // consume final }

			// Don't expect another closing brace for JSON data
			return &ResourceProperties{
				Name:       name,
				Kind:       kind,
				Properties: nil,
				JsonData:   jsonData,
			}, nil
		} else {
			// Regular property parsing
			props, err := p.parseProperties()
			if err != nil {
				return nil, err
			}
			properties = props
		}

		if p.current.Type != RBRACE {
			return nil, fmt.Errorf("expected }, got \"%v\"", p.current.Literal)
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
	p.pos++
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
			return nil, nil, fmt.Errorf("expected relationship end token, got \"%v\"", p.current.Literal)
		}
	case REL_BEGINPROPS_NONE:
		direction = Right
		p.advance()
		var err error
		resourceProps, err = p.parseRelationshipProperties()
		if err != nil {
			return nil, nil, err
		}
		if p.current.Type != REL_ENDPROPS_RIGHT {
			return nil, nil, fmt.Errorf("expected relationship end token, got \"%v\"", p.current.Literal)
		}
	default:
		return nil, nil, fmt.Errorf("unexpected relationship token: \"%v\"", p.current.Literal)
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

// parseSetClause parses: SET KeyValuePairs
func (p *Parser) parseSetClause() (*SetClause, error) {
	if p.current.Type != SET {
		return nil, fmt.Errorf("expected SET, got \"%v\"", p.current.Literal)
	}
	p.advance()

	filters, err := p.parseFilters()
	if err != nil {
		return nil, err
	}

	// Extract only KeyValuePairs for SET clause
	kvPairs := extractKeyValuePairs(filters)
	return &SetClause{KeyValuePairs: kvPairs}, nil
}

// parseDeleteClause parses: DELETE NodeIds
func (p *Parser) parseDeleteClause() (*DeleteClause, error) {
	if p.current.Type != DELETE {
		return nil, fmt.Errorf("expected DELETE, got \"%v\"", p.current.Literal)
	}
	p.advance()

	var nodeIds []string
	for {
		if p.current.Type != IDENT {
			return nil, fmt.Errorf("expected identifier, got \"%v\"", p.current.Literal)
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

// parseReturnClause parses: RETURN ReturnItems (ORDER BY OrderByItems)? (LIMIT Number)? ((SKIP | OFFSET) Number)?
func (p *Parser) parseReturnClause() (*ReturnClause, error) {
	if p.current.Type != RETURN {
		return nil, fmt.Errorf("expected RETURN, got \"%v\"", p.current.Literal)
	}
	p.advance()

	items, err := p.parseReturnItems()
	if err != nil {
		return nil, err
	}

	returnClause := &ReturnClause{
		Items: items,
	}

	// Parse optional ORDER BY clause
	if p.current.Type == ORDER {
		p.advance()
		if p.current.Type != BY {
			return nil, fmt.Errorf("expected BY after ORDER, got \"%v\"", p.current.Literal)
		}
		p.advance()

		orderByItems, err := p.parseOrderByItems()
		if err != nil {
			return nil, err
		}
		returnClause.OrderBy = orderByItems
	}

	// Parse optional LIMIT clause
	if p.current.Type == LIMIT {
		p.advance()
		limit, err := p.parseNumber("LIMIT")
		if err != nil {
			return nil, err
		}
		returnClause.Limit = limit
	}

	// Parse optional SKIP/OFFSET clause
	if p.current.Type == SKIP || p.current.Type == OFFSET {
		keyword := p.current.Literal
		p.advance()
		skip, err := p.parseNumber(keyword)
		if err != nil {
			return nil, err
		}
		returnClause.Skip = skip
	}

	return returnClause, nil
}

// parseNumber parses a potentially signed number and validates it's positive
func (p *Parser) parseNumber(keyword string) (int, error) {
	// Handle negative sign
	isNegative := false
	if p.current.Type == MINUS {
		isNegative = true
		p.advance()
	}

	if p.current.Type != NUMBER {
		return 0, fmt.Errorf("expected number after %s, got \"%v\"", keyword, p.current.Literal)
	}

	num, err := strconv.Atoi(p.current.Literal)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value: %v", keyword, err)
	}

	if isNegative {
		num = -num
	}

	if num < 0 {
		return 0, fmt.Errorf("%s must be a positive number", keyword)
	}

	p.advance()
	return num, nil
}

// parseReturnItems parses a list of return items
func (p *Parser) parseReturnItems() ([]*ReturnItem, error) {
	var items []*ReturnItem

	for {
		var item ReturnItem

		// Check for aggregation functions
		if p.current.Type == COUNT || p.current.Type == SUM {
			item.Aggregate = strings.ToUpper(p.current.Literal)
			p.advance()

			if p.current.Type != LBRACE {
				return nil, fmt.Errorf("expected {, got \"%v\"", p.current.Literal)
			}
			p.advance()
		}

		// Parse node reference
		if p.current.Type != IDENT {
			return nil, fmt.Errorf("expected identifier, got \"%v\"", p.current.Literal)
		}
		nodeRef := p.current.Literal
		p.advance()

		// Handle full node reference or path
		if p.current.Type == DOT {
			p.advance()
			var path strings.Builder
			path.WriteString(nodeRef)
			path.WriteString(".")

			for {
				if p.current.Type != IDENT {
					return nil, fmt.Errorf("expected identifier, got \"%v\"", p.current.Literal)
				}
				path.WriteString(p.current.Literal)
				p.advance()

				// Handle array indices and dots
				if p.current.Type == LBRACKET {
					p.advance()
					path.WriteString("[")

					// Handle wildcard [*]
					if p.current.Type == ILLEGAL && p.current.Literal == "*" {
						path.WriteString("*")
						p.advance()
					} else if p.current.Type != NUMBER {
						return nil, fmt.Errorf("expected number or * in array index, got \"%v\"", p.current.Literal)
					} else {
						path.WriteString(p.current.Literal)
						p.advance()
					}

					if p.current.Type != RBRACKET {
						return nil, fmt.Errorf("expected closing bracket, got \"%v\"", p.current.Literal)
					}
					path.WriteString("]")
					p.advance()
				}

				if p.current.Type != DOT {
					break
				}
				p.advance()
				path.WriteString(".")
			}
			item.JsonPath = path.String()
		} else {
			item.JsonPath = nodeRef
		}

		// Handle closing brace for aggregation
		if item.Aggregate != "" {
			if p.current.Type != RBRACE {
				return nil, fmt.Errorf("expected }, got \"%v\"", p.current.Literal)
			}
			p.advance()
		}

		// Check for AS alias
		if p.current.Type == AS {
			p.advance()
			if p.current.Type != IDENT {
				return nil, fmt.Errorf("expected identifier after AS, got \"%v\"", p.current.Literal)
			}
			item.Alias = p.current.Literal
			p.advance()
		}

		items = append(items, &item)

		if p.current.Type != COMMA {
			break
		}
		p.advance()
	}

	return items, nil
}

// parseContexts parses a list of context identifiers
func (p *Parser) parseContexts() ([]string, error) {
	var contexts []string
	var currentContext strings.Builder

	for {
		if p.current.Type != IDENT {
			return nil, fmt.Errorf("expected identifier, got \"%v\"", p.current.Literal)
		}

		// Start building the context name
		currentContext.WriteString(p.current.Literal)
		p.advance()

		// Handle dashed names specifically in contexts
		for p.current.Type == IDENT && p.current.Literal == "-" {
			currentContext.WriteString("-")
			p.advance()

			if p.current.Type != IDENT {
				return nil, fmt.Errorf("expected identifier after dash, got \"%v\"", p.current.Literal)
			}
			currentContext.WriteString(p.current.Literal)
			p.advance()
		}

		contexts = append(contexts, currentContext.String())
		currentContext.Reset()

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
		debugLog("Parsing property, current token: type=%v literal='%s'", p.current.Type, p.current.Literal)

		if p.current.Type != IDENT && p.current.Type != STRING {
			return nil, fmt.Errorf("expected property key, got \"%v\"", p.current.Literal)
		}

		// Always trim quotes from property keys
		key := strings.Trim(p.current.Literal, "\"")
		debugLog("Property key after trim: '%s'", key)

		p.advance()

		if p.current.Type != COLON {
			return nil, fmt.Errorf("expected :, got \"%v\"", p.current.Literal)
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
		debugLog("Added property: key='%s' value='%v'", key, value)

		if p.current.Type != COMMA {
			break
		}
		p.advance()
	}

	return &Properties{PropertyList: propertyList}, nil
}

// parseFilters parses a list of either key-value pairs with operators or submatch patterns
func (p *Parser) parseFilters() ([]*Filter, error) {
	var filters []*Filter

	for {
		// Check for NOT token before the pattern
		isNegated := false
		if p.current.Type == NOT {
			isNegated = true
			p.advance()
		}

		// Check if this is a submatch pattern
		if p.current.Type == LPAREN {
			// Check if there are any kindless nodes in the match clause
			if hasKindlessNodes(p.matchNodes) {
				return nil, fmt.Errorf("pattern-based filters in WHERE clause are not allowed when kindless nodes exist in the MATCH clause")
			}

			nodeRels, err := p.parseNodeRelationshipList()
			if err != nil {
				return nil, err
			}

			// Validate the submatch pattern
			err = p.validateSubmatchPattern(nodeRels.Nodes, nodeRels.Relationships)
			if err != nil {
				return nil, err
			}

			var referenceNodeName string
			for _, node := range nodeRels.Nodes {
				if !strings.Contains(node.ResourceProperties.Name, "_anon") {
					referenceNodeName = node.ResourceProperties.Name
					break
				}
			}

			filters = append(filters, &Filter{
				Type: "SubMatch",
				SubMatch: &SubMatch{
					IsNegated:         isNegated,
					Nodes:             nodeRels.Nodes,
					Relationships:     nodeRels.Relationships,
					ReferenceNodeName: referenceNodeName,
				},
			})
		} else {
			// Handle regular key-value pair
			if p.current.Type != IDENT {
				return nil, fmt.Errorf("expected identifier or pattern, got \"%v\"", p.current.Literal)
			}
			var path strings.Builder
			path.WriteString(p.current.Literal)
			p.advance()

			for {
				if p.current.Type == DOT {
					p.advance()
					path.WriteString(".")
					if p.current.Type != IDENT {
						return nil, fmt.Errorf("expected identifier after dot, got \"%v\"", p.current.Literal)
					}
					path.WriteString(p.current.Literal)
					p.advance()
				} else if p.current.Type == LBRACKET {
					p.advance()
					path.WriteString("[")
					// Add support for wildcard
					if p.current.Type == ILLEGAL && p.current.Literal == "*" {
						path.WriteString("*")
						p.advance()
					} else if p.current.Type == NUMBER {
						path.WriteString(p.current.Literal)
						p.advance()
					} else {
						return nil, fmt.Errorf("expected number or * in array index, got \"%v\"", p.current.Literal)
					}
					if p.current.Type != RBRACKET {
						return nil, fmt.Errorf("expected closing bracket, got \"%v\"", p.current.Literal)
					}
					path.WriteString("]")
					p.advance()
					if p.current.Type == DOT {
						continue
					}
				} else {
					break
				}
			}

			operator, err := p.parseOperator()
			if err != nil {
				return nil, err
			}

			value, err := p.parseValue()
			if err != nil {
				return nil, err
			}

			filters = append(filters, &Filter{
				Type: "KeyValuePair",
				KeyValuePair: &KeyValuePair{
					Key:       path.String(),
					Value:     value,
					Operator:  operator,
					IsNegated: isNegated,
				},
			})
		}

		if p.current.Type != COMMA && p.current.Type != AND {
			break
		}
		p.advance()
	}

	return filters, nil
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
		return "", fmt.Errorf("expected operator, got \"%v\"", p.current.Literal)
	}
}

// parseValue parses literal values (string, int, boolean, jsondata, null, or temporal expressions)
func (p *Parser) parseValue() (interface{}, error) {
	switch p.current.Type {
	case STRING:
		value := strings.Trim(p.current.Literal, "\"")
		p.advance()
		return value, nil
	case INT, NUMBER:
		value := p.current.Literal
		intVal, err := strconv.Atoi(value)
		if err == nil {
			p.advance()
			return intVal, nil
		}
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
	case DATETIME, DURATION:
		return p.parseTemporalExpression()
	case LBRACE:
		// Collect all tokens until matching }
		var jsonBuilder strings.Builder
		jsonBuilder.WriteString("{")
		depth := 1
		p.advance()

		for depth > 0 {
			if p.current.Type == EOF {
				return nil, fmt.Errorf("unexpected EOF in JSON data")
			}
			jsonBuilder.WriteString(p.current.Literal)
			if p.current.Type == LBRACE {
				depth++
			}
			if p.current.Type == RBRACE {
				depth--
			}
			if depth > 0 {
				p.advance()
			}
		}
		p.advance() // consume final }
		return jsonBuilder.String(), nil
	default:
		return nil, fmt.Errorf("expected value, got \"%v\"", p.current.Literal)
	}
}

// parseTemporalExpression parses datetime() and duration() functions and their operations
func (p *Parser) parseTemporalExpression() (*TemporalExpression, error) {
	// Save the function type
	if p.current.Type != DATETIME && p.current.Type != DURATION {
		return nil, fmt.Errorf("expected datetime or duration function, got \"%v\"", p.current.Literal)
	}
	function := p.current.Literal
	p.advance()

	// Expect opening parenthesis
	if p.current.Type != LPAREN {
		return nil, fmt.Errorf("expected (, got \"%v\"", p.current.Literal)
	}
	p.advance()

	var argument string
	// Parse argument if present
	if p.current.Type == STRING {
		argument = strings.Trim(p.current.Literal, "\"")
		p.advance()

		// Validate the argument format based on function type
		if function == "duration" {
			if !isValidISO8601Duration(argument) {
				return nil, fmt.Errorf("invalid ISO 8601 duration format: \"%v\"", argument)
			}
		} else if function == "datetime" {
			if _, err := time.Parse(time.RFC3339, argument); err != nil {
				return nil, fmt.Errorf("invalid ISO 8601 datetime format: \"%v\"", argument)
			}
		}
	} else if function == "duration" {
		return nil, fmt.Errorf("duration function requires an ISO 8601 duration string argument")
	}

	// Expect closing parenthesis
	if p.current.Type != RPAREN {
		return nil, fmt.Errorf("expected ), got \"%v\"", p.current.Literal)
	}
	p.advance()

	expr := &TemporalExpression{
		Function: function,
		Argument: argument,
	}

	// Check for temporal operations
	if p.current.Type == PLUS || p.current.Type == MINUS {
		operation := p.current.Literal
		p.advance()

		// Parse the right side of the operation
		if p.current.Type != DATETIME && p.current.Type != DURATION {
			return nil, fmt.Errorf("expected datetime or duration function, got \"%v\"", p.current.Literal)
		}

		rightExpr, err := p.parseTemporalExpression()
		if err != nil {
			return nil, err
		}

		// Validate the operation
		if function == "duration" && rightExpr.Function == "datetime" {
			return nil, fmt.Errorf("invalid temporal expression: duration cannot be subtracted from datetime")
		}

		expr.Operation = operation
		expr.RightExpr = rightExpr
	}

	return expr, nil
}

// isValidISO8601Duration validates that a string is a valid ISO 8601 duration
func isValidISO8601Duration(duration string) bool {
	// Basic validation - should start with P and contain at least one number and designator
	if !strings.HasPrefix(duration, "P") {
		return false
	}

	// Remove the P prefix
	duration = duration[1:]

	// Split into date and time parts if T is present
	var datePart, timePart string
	if idx := strings.Index(duration, "T"); idx != -1 {
		datePart = duration[:idx]
		timePart = duration[idx+1:]
	} else {
		datePart = duration
	}

	// Check date part
	if datePart != "" {
		if !validateDurationPart(datePart, "YMD") {
			return false
		}
	}

	// Check time part
	if timePart != "" {
		if !validateDurationPart(timePart, "HMS") {
			return false
		}
	}

	return true
}

// validateDurationPart validates a part of the duration string against allowed designators
func validateDurationPart(part string, allowedDesignators string) bool {
	var number strings.Builder
	for _, c := range part {
		if c >= '0' && c <= '9' {
			number.WriteRune(c)
		} else if strings.ContainsRune(allowedDesignators, c) {
			if number.Len() == 0 {
				return false // No number before designator
			}
			number.Reset()
		} else {
			return false // Invalid character
		}
	}
	return number.Len() == 0 // Should end with a designator
}

// parseRelationshipProperties parses the properties of a relationship
func (p *Parser) parseRelationshipProperties() (*ResourceProperties, error) {
	if p.current.Type != IDENT {
		return nil, fmt.Errorf("expected identifier, got \"%v\"", p.current.Literal)
	}
	name := p.current.Literal
	p.advance()

	if p.current.Type != COLON {
		return nil, fmt.Errorf("expected :, got \"%v\"", p.current.Literal)
	}
	p.advance()

	if p.current.Type != IDENT {
		return nil, fmt.Errorf("expected kind identifier, got \"%v\"", p.current.Literal)
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
			return nil, fmt.Errorf("expected }, got \"%v\"", p.current.Literal)
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

// ParseQuery is the main entry point for parsing Cyphernetes queries
func ParseQuery(query string) (*Expression, error) {
	parser := NewParser(query)
	expr, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return expr, nil
}

// Add debug logging function
func debugLog(format string, args ...interface{}) {
	if LogLevel == "debug" {
		log.Printf(format, args...)
	}
}

// Add helper method to generate anonymous variable names
func (p *Parser) nextAnonymousVar() string {
	p.anonymousCounter++
	return fmt.Sprintf("_anon%d", p.anonymousCounter)
}

// ValidateAnonymousNode checks if an anonymous node is valid (not standalone)
func ValidateAnonymousNode(node *NodePattern, relationships []*Relationship) error {
	if !node.IsAnonymous {
		return nil // Not an anonymous node, no validation needed
	}

	// Check if the node appears in any relationship
	for _, rel := range relationships {
		if rel.LeftNode.ResourceProperties.Name == node.ResourceProperties.Name ||
			rel.RightNode.ResourceProperties.Name == node.ResourceProperties.Name {
			return nil // Node is used in a relationship, valid
		}
	}

	// Node is anonymous and not used in any relationship
	return fmt.Errorf("standalone anonymous node '()' is not allowed")
}

// Add after parseMatchClause
func (p *Parser) trackMatchVariables(nodes []*NodePattern) {
	for _, node := range nodes {
		if !node.IsAnonymous && node.ResourceProperties != nil {
			p.matchVariables[node.ResourceProperties.Name] = node
		}
	}
}

// Add new method to validate submatch patterns
func (p *Parser) validateSubmatchPattern(nodes []*NodePattern, relationships []*Relationship) error {
	referenceCount := 0
	var referenceNode *NodePattern

	// First pass: Find and validate reference nodes
	for _, node := range nodes {
		if !node.IsAnonymous {
			if originalNode, exists := p.matchVariables[node.ResourceProperties.Name]; exists {
				referenceCount++
				referenceNode = node

				// Validate reference node doesn't have kind or properties
				if node.ResourceProperties.Kind != "" {
					return fmt.Errorf("reference node cannot have a kind")
				}
				if node.ResourceProperties.Properties != nil {
					return fmt.Errorf("reference node cannot have properties")
				}

				// Copy kind from original node for later use
				node.ResourceProperties.Kind = originalNode.ResourceProperties.Kind
			}
		}
	}

	// Validate reference count
	if referenceCount == 0 {
		return fmt.Errorf("pattern must reference exactly one variable from the MATCH clause")
	}
	if referenceCount > 1 {
		return fmt.Errorf("pattern must reference exactly one variable from the MATCH clause, found %d", referenceCount)
	}

	// Second pass: Validate other nodes
	for _, node := range nodes {
		if node != referenceNode && !node.IsAnonymous {
			return fmt.Errorf("node '%s' in WHERE pattern must not specify a variable name", node.ResourceProperties.Name)
		}
	}

	// TODO: Validate a relationship rule exists between the reference node and the other nodes
	debugLog("relationhships in validateSubmatchPattern: %v", relationships)

	return nil
}

// Add new method to extract KeyValuePairs from Filters
func extractKeyValuePairs(filters []*Filter) []*KeyValuePair {
	var kvPairs []*KeyValuePair
	for _, filter := range filters {
		if filter.Type == "KeyValuePair" {
			kvPairs = append(kvPairs, filter.KeyValuePair)
		}
	}
	return kvPairs
}

// Add helper function to check for kindless nodes
func hasKindlessNodes(nodes []*NodePattern) bool {
	for _, node := range nodes {
		if node.ResourceProperties != nil && node.ResourceProperties.Kind == "" {
			return true
		}
	}
	return false
}

// parseOrderByItems parses a list of ORDER BY items
func (p *Parser) parseOrderByItems() ([]*OrderByItem, error) {
	var items []*OrderByItem

	for {
		// Parse the json path
		if p.current.Type != IDENT {
			return nil, fmt.Errorf("expected identifier after ORDER BY, got \"%v\"", p.current.Literal)
		}

		// Build the json path
		var path strings.Builder
		path.WriteString(p.current.Literal)
		p.advance()

		// Check if the variable exists in the match clause
		varName := strings.Split(path.String(), ".")[0]
		if _, exists := p.matchVariables[varName]; !exists {
			return nil, fmt.Errorf("undefined variable in ORDER BY: %s", varName)
		}

		// Parse the rest of the path if any
		for p.current.Type == DOT {
			p.advance()
			if p.current.Type != IDENT {
				return nil, fmt.Errorf("expected identifier after dot, got \"%v\"", p.current.Literal)
			}
			path.WriteString(".")
			path.WriteString(p.current.Literal)
			p.advance()
		}

		// Check for DESC modifier
		desc := false
		if p.current.Type == DESC {
			desc = true
			p.advance()
		}

		items = append(items, &OrderByItem{
			JsonPath: path.String(),
			Desc:     desc,
		})

		if p.current.Type != COMMA {
			break
		}
		p.advance()
	}

	return items, nil
}
