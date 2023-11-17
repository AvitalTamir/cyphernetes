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
    strVal            string
    jsonPath          string
    jsonPathList      []string
    pattern           *NodePattern
    clause            *Clause
    expression        *Expression
    matchClause       *MatchClause
    nodePatternList   []*NodePattern
    returnClause      *ReturnClause
    properties        *Properties
    jsonPathValue     *Property
    jsonPathValueList []*Property
    value             interface{}
}

%token <strVal> IDENT
%token <strVal> JSONPATH
%token <strVal> INT
%token <strVal> BOOLEAN
%token <strVal> STRING
%token LPAREN RPAREN COLON MATCH RETURN EOF LBRACE RBRACE COMMA DASH ARROW_LEFT ARROW_RIGHT

%type<expression> Expression
%type<matchClause> MatchClause
%type<nodePatternList> NodePatternList
%type<returnClause> ReturnClause
%type<pattern> NodePattern
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

%%

Expression:
    MatchClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2}}
    }
;

MatchClause:
    MATCH NodePatternList {
        $$ = &MatchClause{NodePatternList: $2}
    }
;

NodePatternList:
    NodePattern COMMA NodePatternList {
        $$ = append([]*NodePattern{$1}, $3...)
    }
    | NodePattern Relationship NodePattern Relationship NodePatternList {
        $$ = []*NodePattern{$1, $3}
        $1.ConnectedNodePatternRight = &NodePattern{Name: $3.Name, Kind: $3.Kind} // Linking LeftNode and RightNode through Relationship
        $3.ConnectedNodePatternLeft = &NodePattern{Name: $1.Name, Kind: $1.Kind} // For bidirectional relationships
        $3.ConnectedNodePatternRight = &NodePattern{Name: $5[0].Name, Kind: $5[0].Kind} // For bidirectional relationships
        $5[0].ConnectedNodePatternLeft = &NodePattern{Name: $3.Name, Kind: $3.Kind} // Linking RightNode and LeftNode through Relationship
        $$ = append($$, $5...)
    }
    | NodePattern Relationship NodePattern COMMA NodePatternList {
        $$ = []*NodePattern{$1, $3}
        $1.ConnectedNodePatternRight = &NodePattern{Name: $3.Name, Kind: $3.Kind} // Linking LeftNode and RightNode through Relationship
        $3.ConnectedNodePatternLeft = &NodePattern{Name: $1.Name, Kind: $1.Kind} // For bidirectional relationships
        $$ = append($$, $5...)
    }
    | NodePattern Relationship NodePattern {
        $$ = []*NodePattern{$1, $3}
        $1.ConnectedNodePatternRight = &NodePattern{Name: $3.Name, Kind: $3.Kind} // Linking LeftNode and RightNode through Relationship
        $3.ConnectedNodePatternLeft = &NodePattern{Name: $1.Name, Kind: $1.Kind} // For bidirectional relationships
    }
    | NodePattern {
        $$ = []*NodePattern{$1}
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
    DASH DASH { logDebug("Found relationship (no direction)") }
|   ARROW_LEFT { logDebug("Found relationship (left)") }
|   ARROW_RIGHT { logDebug("Found relationship (right)") }
|   ARROW_LEFT ARROW_RIGHT { logDebug("Found relationship (both)") }
;

NodePattern:
    LPAREN IDENT COLON IDENT RPAREN {
        $$ = &NodePattern{Name: $2, Kind: $4, Properties: nil}
    }
    | LPAREN IDENT COLON IDENT Properties RPAREN {
        $$ = &NodePattern{Name: $2, Kind: $4, Properties: $5}
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
