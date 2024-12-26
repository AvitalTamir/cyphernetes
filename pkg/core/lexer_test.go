package core

import (
	"testing"
)

func TestLexer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Token
	}{
		{
			name:  "keywords",
			input: "MATCH CREATE WHERE SET DELETE RETURN IN AS COUNT SUM",
			expected: []Token{
				{Type: MATCH, Literal: "MATCH"},
				{Type: CREATE, Literal: "CREATE"},
				{Type: WHERE, Literal: "WHERE"},
				{Type: SET, Literal: "SET"},
				{Type: DELETE, Literal: "DELETE"},
				{Type: RETURN, Literal: "RETURN"},
				{Type: IN, Literal: "IN"},
				{Type: AS, Literal: "AS"},
				{Type: COUNT, Literal: "COUNT"},
				{Type: SUM, Literal: "SUM"},
				{Type: EOF, Literal: ""},
			},
		},
		{
			name:  "identifiers and literals",
			input: `pod nginx "hello world" 42 true false null`,
			expected: []Token{
				{Type: IDENT, Literal: "pod"},
				{Type: IDENT, Literal: "nginx"},
				{Type: STRING, Literal: `"hello world"`},
				{Type: NUMBER, Literal: "42"},
				{Type: BOOLEAN, Literal: "true"},
				{Type: BOOLEAN, Literal: "false"},
				{Type: NULL, Literal: "null"},
				{Type: EOF, Literal: ""},
			},
		},
		{
			name:  "delimiters",
			input: "( ) { } [ ] : , .",
			expected: []Token{
				{Type: LPAREN, Literal: "("},
				{Type: RPAREN, Literal: ")"},
				{Type: LBRACE, Literal: "{"},
				{Type: RBRACE, Literal: "}"},
				{Type: LBRACKET, Literal: "["},
				{Type: RBRACKET, Literal: "]"},
				{Type: COLON, Literal: ":"},
				{Type: COMMA, Literal: ","},
				{Type: DOT, Literal: "."},
				{Type: EOF, Literal: ""},
			},
		},
		{
			name:  "operators",
			input: "= != > < >= <= =~ CONTAINS",
			expected: []Token{
				{Type: EQUALS, Literal: "="},
				{Type: NOT_EQUALS, Literal: "!="},
				{Type: GREATER_THAN, Literal: ">"},
				{Type: LESS_THAN, Literal: "<"},
				{Type: GREATER_THAN_EQUALS, Literal: ">="},
				{Type: LESS_THAN_EQUALS, Literal: "<="},
				{Type: REGEX_COMPARE, Literal: "=~"},
				{Type: CONTAINS, Literal: "CONTAINS"},
				{Type: EOF, Literal: ""},
			},
		},
		{
			name:  "relationship tokens",
			input: "-> <- -- -[r:uses]-> <-[r:uses]- -[r:uses]-",
			expected: []Token{
				{Type: REL_NOPROPS_RIGHT, Literal: "->"},
				{Type: REL_NOPROPS_LEFT, Literal: "<-"},
				{Type: REL_NOPROPS_NONE, Literal: "--"},
				{Type: REL_BEGINPROPS_NONE, Literal: "-["},
				{Type: IDENT, Literal: "r"},
				{Type: COLON, Literal: ":"},
				{Type: IDENT, Literal: "uses"},
				{Type: REL_ENDPROPS_RIGHT, Literal: "]->"},
				{Type: REL_BEGINPROPS_LEFT, Literal: "<-["},
				{Type: IDENT, Literal: "r"},
				{Type: COLON, Literal: ":"},
				{Type: IDENT, Literal: "uses"},
				{Type: REL_ENDPROPS_NONE, Literal: "]-"},
				{Type: REL_BEGINPROPS_NONE, Literal: "-["},
				{Type: IDENT, Literal: "r"},
				{Type: COLON, Literal: ":"},
				{Type: IDENT, Literal: "uses"},
				{Type: REL_ENDPROPS_NONE, Literal: "]-"},
				{Type: EOF, Literal: ""},
			},
		},
		{
			name:  "complex json",
			input: `{"metadata": {"name": "test", "labels": {"app": "nginx"}}}`,
			expected: []Token{
				{Type: LBRACE, Literal: "{"},
				{Type: STRING, Literal: `"metadata"`},
				{Type: COLON, Literal: ":"},
				{Type: LBRACE, Literal: "{"},
				{Type: STRING, Literal: `"name"`},
				{Type: COLON, Literal: ":"},
				{Type: STRING, Literal: `"test"`},
				{Type: COMMA, Literal: ","},
				{Type: STRING, Literal: `"labels"`},
				{Type: COLON, Literal: ":"},
				{Type: LBRACE, Literal: "{"},
				{Type: STRING, Literal: `"app"`},
				{Type: COLON, Literal: ":"},
				{Type: STRING, Literal: `"nginx"`},
				{Type: RBRACE, Literal: "}"},
				{Type: RBRACE, Literal: "}"},
				{Type: RBRACE, Literal: "}"},
				{Type: EOF, Literal: ""},
			},
		},
		{
			name:  "invalid tokens",
			input: "@ # $ %",
			expected: []Token{
				{Type: ILLEGAL, Literal: "@"},
				{Type: ILLEGAL, Literal: "#"},
				{Type: ILLEGAL, Literal: "$"},
				{Type: ILLEGAL, Literal: "%"},
				{Type: EOF, Literal: ""},
			},
		},
		{
			name:  "invalid relationship",
			input: "<<",
			expected: []Token{
				{Type: ILLEGAL, Literal: "<<"},
				{Type: EOF, Literal: ""},
			},
		},
		{
			name:  "fully qualified resource kinds",
			input: "(d:deployments.apps), (p:pods.v1.core), (c:configmaps.v1)",
			expected: []Token{
				{Type: LPAREN, Literal: "("},
				{Type: IDENT, Literal: "d"},
				{Type: COLON, Literal: ":"},
				{Type: IDENT, Literal: "deployments.apps"},
				{Type: RPAREN, Literal: ")"},
				{Type: COMMA, Literal: ","},
				{Type: LPAREN, Literal: "("},
				{Type: IDENT, Literal: "p"},
				{Type: COLON, Literal: ":"},
				{Type: IDENT, Literal: "pods.v1.core"},
				{Type: RPAREN, Literal: ")"},
				{Type: COMMA, Literal: ","},
				{Type: LPAREN, Literal: "("},
				{Type: IDENT, Literal: "c"},
				{Type: COLON, Literal: ":"},
				{Type: IDENT, Literal: "configmaps.v1"},
				{Type: RPAREN, Literal: ")"},
				{Type: EOF, Literal: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			for i, expected := range tt.expected {
				got := lexer.NextToken()
				if got.Type != expected.Type || got.Literal != expected.Literal {
					t.Errorf("token[%d] - got=%+v, want=%+v", i, got, expected)
				}
			}
		})
	}
}
