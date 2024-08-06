package parser

import (
	"fmt"
	"log"

	schema "k8s.io/apimachinery/pkg/runtime/schema"
)

var Namespace string
var LogLevel string
var AllNamespaces bool

type Expression struct {
	Clauses []Clause
}

type Clause interface {
	isClause()
}

type MatchClause struct {
	Nodes         []*NodePattern
	Relationships []*Relationship
	ExtraFilters  []*KeyValuePair
}

type SetClause struct {
	KeyValuePairs []*KeyValuePair
}

type DeleteClause struct {
	NodeIds []string
}

type KeyValuePair struct {
	Key   string
	Value interface{}
}

type CreateClause struct {
	Nodes         []*NodePattern
	Relationships []*Relationship
}

type Relationship struct {
	ResourceProperties *ResourceProperties
	Direction          Direction
	LeftNode           *NodePattern
	RightNode          *NodePattern
}

type NodeRelationshipList struct {
	Nodes         []*NodePattern
	Relationships []*Relationship
}

type Direction string

const (
	Left  Direction = "left"
	Right Direction = "right"
	Both  Direction = "both"
	None  Direction = "none"
)

type ReturnClause struct {
	Items []*ReturnItem
}

type ReturnItem struct {
	JsonPath string
	Alias    string
}

type NodePattern struct {
	ResourceProperties *ResourceProperties
}

type ResourceProperties struct {
	Name       string
	Kind       string
	Properties *Properties
	JsonData   string
}

type Properties struct {
	PropertyList []*Property
}

type Property struct {
	Key string
	// Value is string int or bool
	Value interface{}
}

type JSONPathValueList struct {
	JSONPathValues []*JSONPathValue
}

type JSONPathValue struct {
	Value interface{}
}

// Implement isClause for all Clause types
func (m *MatchClause) isClause()  {}
func (s *SetClause) isClause()    {}
func (d *DeleteClause) isClause() {}
func (r *ReturnClause) isClause() {}
func (c *CreateClause) isClause() {}

var result *Expression

func ParseQuery(query string) (*Expression, error) {
	lexer := NewLexer(query)
	if yyParse(lexer) != 0 {
		return nil, fmt.Errorf("parsing failed")
	}

	return result, nil
}

func logDebug(v ...interface{}) {
	if LogLevel == "debug" {
		log.Println(v...)
	}
}

func ClearCache() {
	GvrCacheMutex.Lock()
	GvrCache = make(map[string]schema.GroupVersionResource)
	GvrCacheMutex.Unlock()

	apiResourceListCache = nil

	// Clear the resultCache
	resultCache = make(map[string]interface{})
}

func PrintCache() {
	fmt.Println("GVR Cache:")
	for k, v := range GvrCache {
		fmt.Printf("%s: %s\n", k, v)
	}
	fmt.Println("API Resource List Cache:")
	for _, v := range apiResourceListCache {
		fmt.Printf("%s\n", v)
	}
	fmt.Println("Result Cache:")
	for k, v := range resultCache {
		fmt.Printf("%s: %s\n", k, v)
	}
	fmt.Println("Result Map:")
	for k, v := range resultMap {
		fmt.Printf("%s: %s\n", k, v)
	}
}
