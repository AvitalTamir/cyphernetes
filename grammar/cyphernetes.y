%{
package parser

import (
    "fmt"
    "log"
    "strings"
    "strconv"
)

func yyerror(s string) {
    fmt.Printf("Syntax error: %s\n", s)
}

func debugLog(v ...interface{}) {
	if LogLevel == "debug" {
		log.Println(v...)
	}
}
%}

%union {
    strVal                 string
    jsonPath               string
    nodePattern            *NodePattern
    clause                 *Clause
    expression             *Expression
    matchClause            *MatchClause
    setClause              *SetClause
    deleteClause           *DeleteClause
    createClause           *CreateClause
    returnClause           *ReturnClause
    returnItems            []*ReturnItem
    returnItem             *ReturnItem
    properties             *Properties
    jsonPathValue          *Property
    jsonPathValueList      []*Property
    keyValuePairs          []*KeyValuePair
    keyValuePair           *KeyValuePair
    value                  interface{}
    relationship           *Relationship
    resourceProperties     *ResourceProperties
    nodeRelationshipList   *NodeRelationshipList
    nodeIds        []string
    contexts              []string
}

%token <strVal> IDENT
%token <strVal> JSONPATH
%token <strVal> INT
%token <strVal> BOOLEAN
%token <strVal> STRING
%token <strVal> JSONDATA
%token LPAREN RPAREN COLON MATCH WHERE SET DELETE CREATE RETURN EOF LBRACE RBRACE COMMA EQUALS AS
%token REL_NOPROPS_RIGHT REL_NOPROPS_LEFT REL_NOPROPS_BOTH REL_NOPROPS_NONE REL_BEGINPROPS_LEFT REL_BEGINPROPS_NONE REL_ENDPROPS_RIGHT REL_ENDPROPS_NONE
%token COUNT SUM NOT_EQUALS GREATER_THAN LESS_THAN GREATER_THAN_EQUALS LESS_THAN_EQUALS CONTAINS REGEX_COMPARE
%token IN
%token NULL

%type<expression> Expression
%type<matchClause> MatchClause
%type<setClause> SetClause
%type<deleteClause> DeleteClause
%type<createClause> CreateClause
%type<returnClause> ReturnClause
%type<nodePattern> NodePattern
%type<strVal> IDENT
%type<strVal> JSONPATH
%type<jsonPathValueList> JSONPathValueList
%type<jsonPathValue> JSONPathValue
%type<value> Value
%type<properties> Properties
%type<strVal> STRING
%type<strVal> INT
%type<strVal> BOOLEAN
%type<relationship> Relationship
%type<resourceProperties> ResourceProperties
%type<nodeRelationshipList> NodeRelationshipList
%type<keyValuePairs> KeyValuePairs
%type<keyValuePair> KeyValuePair
%type<nodeIds> NodeIds
%type<returnItems> ReturnItems
%type<returnItem> ReturnItem
%type<contexts> Contexts
%type<strVal> Context

%%

Expression:
    MatchClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2}}
    }
    | IN Contexts MatchClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$3, $4}, Contexts: $2}
    }
    | MatchClause SetClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2}}
    }
    | IN Contexts MatchClause SetClause EOF {
        result = &Expression{Clauses: []Clause{$3, $4}, Contexts: $2}
    }
    | MatchClause SetClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2, $3}}
    }
    | IN Contexts MatchClause SetClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$3, $4, $5}, Contexts: $2}
    }
    | MatchClause DeleteClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2}}
    }
    | IN Contexts MatchClause DeleteClause EOF {
        result = &Expression{Clauses: []Clause{$3, $4}, Contexts: $2}
    }
    | CreateClause EOF {
        result = &Expression{Clauses: []Clause{$1}}
    }
    | IN Contexts CreateClause EOF {
        result = &Expression{Clauses: []Clause{$3}, Contexts: $2}
    }
    | CreateClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2}}
    }
    | IN Contexts CreateClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$3, $4}, Contexts: $2}
    }
    | MatchClause CreateClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2}}
    }
    | IN Contexts MatchClause CreateClause EOF {
        result = &Expression{Clauses: []Clause{$3, $4}, Contexts: $2}
    }
    | MatchClause CreateClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2, $3}}
    }
    | IN Contexts MatchClause CreateClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$3, $4, $5}, Contexts: $2}
    }
;

MatchClause:
    MATCH NodeRelationshipList {
        $$ = &MatchClause{Nodes: $2.Nodes, Relationships: $2.Relationships, ExtraFilters: nil}
    }
    | MATCH NodeRelationshipList WHERE KeyValuePairs {
        $$ = &MatchClause{Nodes: $2.Nodes, Relationships: $2.Relationships, ExtraFilters: $4}
    }
;

CreateClause:
    CREATE NodeRelationshipList {
        $$ = &CreateClause{Nodes: $2.Nodes, Relationships: $2.Relationships}
    }
;

SetClause:
    SET KeyValuePairs {
        $$ = &SetClause{KeyValuePairs: $2}
    }
;

DeleteClause:
    DELETE NodeIds {
        $$ = &DeleteClause{NodeIds: $2}
    }
;

NodeIds:
    IDENT {
        $$ = []string{$1}
    }
    | NodeIds COMMA IDENT {
        $$ = append($1, $3)
    }
;

KeyValuePairs:
    KeyValuePair {
        $$ = []*KeyValuePair{$1} // Start with one Property element
    }
    | KeyValuePairs COMMA KeyValuePair {
        $$ = append($1, $3) // $1 and $3 are the left and right operands of COMMA
    }
;

// JSONPathValue represents a JSONPath=Value pair
KeyValuePair:
    JSONPATH EQUALS Value {
        $$ = &KeyValuePair{Key: $1, Value: $3, Operator: "EQUALS"} // ==
    }
    | JSONPATH NOT_EQUALS Value {
        $$ = &KeyValuePair{Key: $1, Value: $3, Operator: "NOT_EQUALS"} // !=
    }
    | JSONPATH GREATER_THAN Value {
        $$ = &KeyValuePair{Key: $1, Value: $3, Operator: "GREATER_THAN"} // >
    }
    | JSONPATH LESS_THAN Value {
        $$ = &KeyValuePair{Key: $1, Value: $3, Operator: "LESS_THAN"} // <
    }
    | JSONPATH GREATER_THAN_EQUALS Value {
        $$ = &KeyValuePair{Key: $1, Value: $3, Operator: "GREATER_THAN_EQUALS"} // >=
    }
    | JSONPATH LESS_THAN_EQUALS Value {
        $$ = &KeyValuePair{Key: $1, Value: $3, Operator: "LESS_THAN_EQUALS"} // <=
    }
    | JSONPATH CONTAINS Value {
        $$ = &KeyValuePair{Key: $1, Value: $3, Operator: "CONTAINS"} // CONTAINS
    }
    | JSONPATH REGEX_COMPARE Value {
        $$ = &KeyValuePair{Key: $1, Value: $3, Operator: "REGEX_COMPARE"} // =~
    }
;

NodeRelationshipList:
    NodePattern {
        $$ = &NodeRelationshipList{
            Nodes:    []*NodePattern{$1},
            Relationships:   []*Relationship{},
        }
    }
    | NodePattern Relationship NodePattern {
        $2.LeftNode = $1
        $2.RightNode = $3
        $$ = &NodeRelationshipList{
            Nodes:    []*NodePattern{$1, $3},
            Relationships:   []*Relationship{$2},
        }
    }
    | NodePattern Relationship NodePattern COMMA NodeRelationshipList {
        $2.LeftNode = $1
        $2.RightNode = $3
        $$ = &NodeRelationshipList{
            Nodes:    append([]*NodePattern{$1, $3}, $5.Nodes...),
            Relationships:   append([]*Relationship{$2}, $5.Relationships...),
        }
    }
    | NodePattern Relationship NodePattern Relationship NodeRelationshipList {
        $2.LeftNode = $1
        $2.RightNode = $3
        $4.LeftNode = $3
        $4.RightNode = $5.Nodes[0]
        $$ = &NodeRelationshipList{
            Nodes:    append([]*NodePattern{$1, $3}, $5.Nodes...),
            Relationships:   append([]*Relationship{$2, $4}, $5.Relationships...),
        }
    }
    | NodePattern COMMA NodeRelationshipList {
        $$ = &NodeRelationshipList{
            Nodes:    append([]*NodePattern{$1}, $3.Nodes...),
            Relationships:   $3.Relationships,
        }
    }
;

NodePattern:
    LPAREN ResourceProperties RPAREN {
        $$ = &NodePattern{ResourceProperties: $2}
    }
    | LPAREN IDENT RPAREN {
        $$ = &NodePattern{&ResourceProperties{Name: $2, Kind: "", Properties: nil, JsonData: ""}}
    }
;

ReturnClause:
    RETURN ReturnItems {
        $$ = &ReturnClause{Items: $2}
    }
;

ReturnItems:
    ReturnItem {
        $$ = []*ReturnItem{$1}
    }
    | ReturnItems COMMA ReturnItem {
        $$ = append($1, $3)
    }
;

ReturnItem:
    JSONPATH {
        $$ = &ReturnItem{JsonPath: $1}
    }
    | JSONPATH AS IDENT {
        $$ = &ReturnItem{JsonPath: $1, Alias: $3}
    }
    | COUNT LBRACE JSONPATH RBRACE {
        $$ = &ReturnItem{Aggregate: "COUNT", JsonPath: $3}
    }
    | SUM LBRACE JSONPATH RBRACE {
        $$ = &ReturnItem{Aggregate: "SUM", JsonPath: $3}
    }
    | COUNT LBRACE JSONPATH RBRACE AS IDENT {
        $$ = &ReturnItem{Aggregate: "COUNT", JsonPath: $3, Alias: $6}
    }
    | SUM LBRACE JSONPATH RBRACE AS IDENT {
        $$ = &ReturnItem{Aggregate: "SUM", JsonPath: $3, Alias: $6}
    }
;

Relationship:
    REL_NOPROPS_NONE { 
        $$ = &Relationship{ResourceProperties: nil, Direction: None, LeftNode: nil, RightNode: nil}
    }
|   REL_NOPROPS_LEFT { 
        $$ = &Relationship{ResourceProperties: nil, Direction: Left, LeftNode: nil, RightNode: nil}
    }
|   REL_NOPROPS_RIGHT  { 
        $$ = &Relationship{ResourceProperties: nil, Direction: Right, LeftNode: nil, RightNode: nil}
    }
|   REL_NOPROPS_BOTH { 
        $$ = &Relationship{ResourceProperties: nil, Direction: Both, LeftNode: nil, RightNode: nil}
    }
|   REL_BEGINPROPS_NONE ResourceProperties REL_ENDPROPS_NONE { 
        $$ = &Relationship{ResourceProperties: $2, Direction: None, LeftNode: nil, RightNode: nil}
    }
|   REL_BEGINPROPS_LEFT ResourceProperties REL_ENDPROPS_NONE {
        $$ = &Relationship{ResourceProperties: $2, Direction: Left, LeftNode: nil, RightNode: nil}
    }
|   REL_BEGINPROPS_NONE ResourceProperties REL_ENDPROPS_RIGHT { 
        $$ = &Relationship{ResourceProperties: $2, Direction: Right, LeftNode: nil, RightNode: nil}
    }
|   REL_BEGINPROPS_LEFT ResourceProperties REL_ENDPROPS_RIGHT { 
        $$ = &Relationship{ResourceProperties: $2, Direction: Both, LeftNode: nil, RightNode: nil}
    }
;

ResourceProperties:
    IDENT COLON IDENT {
        $$ = &ResourceProperties{Name: $1, Kind: $3, Properties: nil, JsonData: ""}
    }
    | IDENT COLON IDENT Properties  {
        $$ = &ResourceProperties{Name: $1, Kind: $3, Properties: $4, JsonData: ""}
    }
    | IDENT COLON IDENT LBRACE JSONDATA {
        $$ = &ResourceProperties{Name: $1, Kind: $3, Properties: nil, JsonData: $5}
    }
;

Properties:
    LBRACE JSONPathValueList RBRACE {
        $$ = &Properties{PropertyList: $2}
    }
;

JSONPathValueList:
    JSONPathValue {
        $$ = []*Property{$1} // Start with one Property element
    }
    | JSONPathValueList COMMA JSONPathValue {
        $$ = append($1, $3) // $1 and $3 are the left and right operands of COMMA
    }
;

JSONPathValue:
    JSONPATH COLON Value {
        $$ = &Property{Key: $1, Value: $3}
    }
;

Value:
    STRING { 
        $$ = strings.Trim($1, "\"")
    }
    | INT { 
        i, err := strconv.Atoi($1)
        if err != nil {
            panic(err)
        }
        $$ = i
    }
    | BOOLEAN {
        $$ = strings.ToUpper($1) == "TRUE"
    }
    | JSONDATA {
        $$ = $1
    }
    | NULL {
        $$ = nil
    }
;

Contexts:
    Context {
        $$ = []string{$1}
    }
    | Contexts COMMA Context {
        $$ = append($1, $3)
    }
;

Context:
    IDENT {
        $$ = $1
    }
;

%%
