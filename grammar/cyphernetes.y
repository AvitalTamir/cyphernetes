%{
package cmd

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
	if logLevel == "debug" {
		log.Println(v...)
	}
}
%}

%union {
    strVal                 string
    jsonPath               string
    jsonPathList           []string
    nodePattern            *NodePattern
    clause                 *Clause
    expression             *Expression
    matchClause            *MatchClause
    returnClause           *ReturnClause
    properties             *Properties
    jsonPathValue          *Property
    jsonPathValueList      []*Property
    value                  interface{}
    relationship           *Relationship
    resourceProperties     *ResourceProperties
    nodeRelationshipList   *NodeRelationshipList
}

%token <strVal> IDENT
%token <strVal> JSONPATH
%token <strVal> INT
%token <strVal> BOOLEAN
%token <strVal> STRING
%token LPAREN RPAREN COLON MATCH RETURN EOF LBRACE RBRACE COMMA
%token REL_NOPROPS_RIGHT REL_NOPROPS_LEFT REL_NOPROPS_BOTH REL_NOPROPS_NONE REL_BEGINPROPS_LEFT REL_BEGINPROPS_NONE REL_ENDPROPS_RIGHT REL_ENDPROPS_NONE

%type<expression> Expression
%type<matchClause> MatchClause
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
%type<jsonPathList> JSONPathList
%type<relationship> Relationship
%type<resourceProperties> ResourceProperties
%type<nodeRelationshipList> NodeRelationshipList

%%

Expression:
    MatchClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2}}
    }
;

MatchClause:
    MATCH NodeRelationshipList {
        $$ = &MatchClause{Nodes: $2.Nodes, Relationships: $2.Relationships}
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


/* NodePatternList:
    NodePattern COMMA NodePatternList {
        $$ = append([]*NodePattern{$1}, $3...)
    }
    | NodePattern Relationship NodePattern Relationship NodePatternList {
        $$ = []*NodePattern{$1, $3}
        $2.LeftNode = $1
        $2.RightNode = $3
        $4.LeftNode = $3
        $4.RightNode = $5[0]
        $$ = append($$, $5...)
    }
    | NodePattern Relationship NodePattern COMMA NodePatternList {
        $$ = []*NodePattern{$1, $3}
        $2.LeftNode = $1
        $2.RightNode = $3
        $$ = append($$, $5...)
    }
    | NodePattern Relationship NodePattern {
        $$ = []*NodePattern{$1, $3}
        $2.LeftNode = $1
        $2.RightNode = $3
    }
    | NodePattern {
        $$ = []*NodePattern{$1}
    }
; */


NodePattern:
    LPAREN ResourceProperties RPAREN {
        $$ = &NodePattern{ResourceProperties: $2}
    }
;

ReturnClause:
    RETURN JSONPathList {
        $$ = &ReturnClause{JsonPaths: $2}
    }
;

JSONPathList:
    JSONPATH {
        $$ = []string{$1}
    }
|   JSONPathList COMMA JSONPATH {
        $$ = append($1, $3)
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
        $$ = &ResourceProperties{Name: $1, Kind: $3, Properties: nil}
    }
    | IDENT COLON IDENT Properties  {
        $$ = &ResourceProperties{Name: $1, Kind: $3, Properties: $4}
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
        // Parse the int from the string
        i, err := strconv.Atoi($1)
        if err != nil {
            // ... handle error
            panic(err)
        }
        $$ = i
    }
    | BOOLEAN {
        // Parse the boolean from the string
        $$ = strings.ToUpper($1) == "TRUE"
    }
;
%%
