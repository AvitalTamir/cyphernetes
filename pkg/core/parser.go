package core

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

type Parser struct {
	lexer            *Lexer
	current          Token
	pos              int
	inCreate         bool
	anonymousCounter int
}

func NewRecursiveParser(input string) *Parser {
	lexer := NewLexer(input)
	return &Parser{
		lexer:            lexer,
		anonymousCounter: 0,
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
			filters, err := p.parseKeyValuePairs()
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

	// Validate that we don't have standalone anonymous nodes
	if len(nodeRels.Nodes) == 1 && len(nodeRels.Relationships) == 0 {
		node := nodeRels.Nodes[0]
		if node.IsAnonymous || (node.ResourceProperties != nil && node.ResourceProperties.Name == "") {
			return nil, fmt.Errorf("standalone anonymous nodes are not allowed")
		}
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
		if p.current.Type != RPAREN {
			return nil, fmt.Errorf("expected ), got \"%v\"", p.current.Literal)
		}
		p.advance()
	} else {
		if p.current.Type != RPAREN {
			return nil, fmt.Errorf("expected ), got \"%v\"", p.current.Literal)
		}
		p.advance()
	}

	// Initialize resourceProps if it's nil
	if resourceProps == nil {
		resourceProps = &ResourceProperties{
			Name: name,
			Kind: "", // Empty kind for variable-only nodes
		}
	}

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

	pairs, err := p.parseKeyValuePairs()
	if err != nil {
		return nil, err
	}

	return &SetClause{KeyValuePairs: pairs}, nil
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

// parseReturnClause parses: RETURN ReturnItems
func (p *Parser) parseReturnClause() (*ReturnClause, error) {
	if p.current.Type != RETURN {
		return nil, fmt.Errorf("expected RETURN, got \"%v\"", p.current.Literal)
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

// parseKeyValuePairs parses a list of key-value pairs with operators
func (p *Parser) parseKeyValuePairs() ([]*KeyValuePair, error) {
	var pairs []*KeyValuePair

	for {
		if p.current.Type != IDENT {
			return nil, fmt.Errorf("expected identifier, got \"%v\"", p.current.Literal)
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

		pairs = append(pairs, &KeyValuePair{
			Key:      path.String(),
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
		return "", fmt.Errorf("expected operator, got \"%v\"", p.current.Literal)
	}
}

// parseValue parses literal values (string, int, boolean, jsondata, or null)
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
	parser := NewRecursiveParser(query)
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
