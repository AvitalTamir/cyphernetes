%{
package cmd

import (
    "fmt"
    "log"
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
    string            string
    int               int
    boolean           bool
}

%token <strVal> IDENT
%token <strVal> JSONPATH
%token LPAREN RPAREN COLON MATCH RETURN EOF STRING INT BOOLEAN LBRACE RBRACE COMMA

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
%type<string> STRING
%type<int> INT
%type<boolean> BOOLEAN

%%

Expression:
    MatchClause ReturnClause EOF {
        result = &Expression{Clauses: []Clause{$1, $2}} // Store the result in a global variable
    }
;

MatchClause:
    MATCH NodePattern {
        debugLog("Parsed MATCH expression for Name:", $2.Name, "Kind:", $2.Kind)
        $$ = &MatchClause{NodePattern: $2}
    }
;

ReturnClause:
    RETURN JSONPATH {
        debugLog("Parsed RETURN expression for JsonPath:", $2)
        $$ = &ReturnClause{JsonPath: $2}
    }
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
    STRING { $$ = $1 }
    | INT { $$ = $1 }
    | BOOLEAN { $$ = $1 }
;
%%
