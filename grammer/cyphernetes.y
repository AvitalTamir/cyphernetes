%{
package cmd

import (
    "fmt"
)

func yyerror(s string) {
    fmt.Printf("Syntax error: %s\n", s)
}

%}

%union {
    strVal  string
    pattern *NodePattern
    clause  *MatchClause
    expression *Expression
}

%token <strVal> IDENT
%token LPAREN RPAREN COLON MATCH EOF

%type<expression> Expression
%type<clause> MatchClause
%type<pattern> NodePattern
%type<strVal> IDENT

%%

Expression:
    MatchClause EOF {
        fmt.Println("Parsed MATCH expression for Name:", $1.NodePattern.Name, "Kind:", $1.NodePattern.Kind)
        result = &Expression{Clauses: []Clause{$1}} // Store the result in a global variable
    }
;

MatchClause:
    MATCH NodePattern {
        $$ = &MatchClause{NodePattern: $2}
    }
;

NodePattern:
    LPAREN IDENT COLON IDENT RPAREN {
        $$ = &NodePattern{Name: $2, Kind: $4}
    }
;

%%
