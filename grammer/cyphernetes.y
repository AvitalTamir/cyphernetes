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
    strVal  string
    pattern *NodePattern
    clause  *Clause
    expression *Expression
    matchClause *MatchClause
    returnClause *ReturnClause
    jsonPath     string
}

%token <strVal> IDENT
%token <strVal> JSONPATH
%token LPAREN RPAREN COLON MATCH RETURN EOF

%type<expression> Expression
%type<matchClause> MatchClause
%type<returnClause> ReturnClause
%type<pattern> NodePattern
%type<strVal> IDENT
%type<strVal> JSONPATH

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
        $$ = &NodePattern{Name: $2, Kind: $4}
    }
;

%%
