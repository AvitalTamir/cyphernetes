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
    pattern           *NodePattern
    clause            *Clause
    expression        *Expression
    matchClause       *MatchClause
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

%%

Expression:
    MatchClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2}} // Store the result in a global variable
    }
;

MatchClause:
    MATCH NodePattern {
        debugLog("Parsed MATCH expression for Name:", $2.Name, "Kind:", $2.Kind)
        $$ = &MatchClause{NodePattern: $2, ConnectedNodePattern: nil}
    }
    | MATCH NodePattern Relationship NodePattern {
        debugLog("Parsed MATCH expression for connected nodes", $2.Name, "and", $4.Name)
        $$ = &MatchClause{NodePattern: $2, ConnectedNodePattern: $4}
    }
;

ReturnClause:
    RETURN JSONPATH {
        debugLog("Parsed RETURN expression for JsonPath:", $2)
        $$ = &ReturnClause{JsonPath: $2}
    }
;

Relationship:
    DASH DASH { debugLog("Found undirectional relationship") } // { $$ = &Relationship{Type: "bidirectional", LeftNode: $1, RightNode: $4} }
|   ARROW_LEFT { debugLog("Found left relationship") } // { $$ = &Relationship{Type: "left", LeftNode: $1, RightNode: $4} }
|   ARROW_RIGHT { debugLog("Found right relationship") } // { $$ = &Relationship{Type: "right", LeftNode: $1, RightNode: $4} }
|   ARROW_LEFT ARROW_RIGHT { debugLog("Found bidirectional relationship") } // { $$ = &Relationship{Type: "both", LeftNode: $1, RightNode: $6} }
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
