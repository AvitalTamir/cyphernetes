package core

// Direction represents the direction of a relationship
type Direction string

const (
	Left  Direction = "left"
	Right Direction = "right"
	Both  Direction = "both"
	None  Direction = "none"
)

// Token represents a lexical token
type Token struct {
	Type    TokenType
	Literal string
}

// TokenType represents the type of a lexical token
type TokenType int

// Expression represents a complete Cyphernetes query
type Expression struct {
	Contexts []string
	Clauses  []Clause
}

// Clause is an interface implemented by all clause types
type Clause interface {
	isClause()
}

// MatchClause represents a MATCH clause
type MatchClause struct {
	Nodes         []*NodePattern
	Relationships []*Relationship
	ExtraFilters  []*Filter
}

// Filter represents a filter condition in a WHERE clause
type Filter struct {
	Type         string        // "KeyValuePair" or "SubMatch"
	KeyValuePair *KeyValuePair // Used when Type is "KeyValuePair"
	SubMatch     *SubMatch     // Used when Type is "SubMatch"
}

// SubMatch represents a pattern match within a WHERE clause
type SubMatch struct {
	IsNegated         bool
	Nodes             []*NodePattern
	Relationships     []*Relationship
	ReferenceNodeName string
}

// CreateClause represents a CREATE clause
type CreateClause struct {
	Nodes         []*NodePattern
	Relationships []*Relationship
}

// SetClause represents a SET clause
type SetClause struct {
	KeyValuePairs []*KeyValuePair
}

// DeleteClause represents a DELETE clause
type DeleteClause struct {
	NodeIds []string
}

// ReturnClause represents a RETURN clause
type ReturnClause struct {
	Items   []*ReturnItem
	OrderBy []*OrderByItem
	Limit   *int
	Skip    *int
}

// ReturnItem represents an item in a RETURN clause
type ReturnItem struct {
	JsonPath  string
	Alias     string
	Aggregate string
}

// OrderByItem represents an ORDER BY field with direction
type OrderByItem struct {
	Field     string // Field name or alias to order by
	Direction string // "ASC" or "DESC"
}

// NodePattern represents a node pattern in a query
type NodePattern struct {
	ResourceProperties *ResourceProperties
	IsAnonymous        bool // Indicates if this is an anonymous node (no variable name)
}

// ResourceProperties represents the properties of a resource
type ResourceProperties struct {
	Name       string
	Kind       string
	Properties *Properties
	JsonData   string
}

// Properties represents a collection of properties
type Properties struct {
	PropertyList []*Property
}

// Property represents a key-value property
type Property struct {
	Key   string
	Value interface{}
}

// KeyValuePair represents a key-value pair with an operator
type KeyValuePair struct {
	Key       string
	Value     interface{}
	Operator  string
	IsNegated bool
}

// TemporalExpression represents a datetime operation (e.g., datetime() - duration("PT1H"))
type TemporalExpression struct {
	Function  string              // "datetime" or "duration"
	Argument  string              // For duration function, the ISO 8601 duration string
	Operation string              // "+" or "-" if this is part of an operation
	RightExpr *TemporalExpression // Right side of the operation, if any
}

// Relationship represents a relationship between nodes
type Relationship struct {
	ResourceProperties *ResourceProperties
	Direction          Direction
	LeftNode           *NodePattern
	RightNode          *NodePattern
}

// NodeRelationshipList represents a list of nodes and relationships
type NodeRelationshipList struct {
	Nodes         []*NodePattern
	Relationships []*Relationship
}

// Implement isClause for all clause types
func (*MatchClause) isClause()  {}
func (*CreateClause) isClause() {}
func (*SetClause) isClause()    {}
func (*DeleteClause) isClause() {}
func (*ReturnClause) isClause() {}
