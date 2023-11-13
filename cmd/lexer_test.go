package cmd

import (
	"strings"
	"testing"
)

func TestLexer(t *testing.T) {
	input := "MATCH (k:Kind)"
	lexer := NewLexer(input)
	lexer.s.Init(strings.NewReader(input))

	expectedTokens := []Token{
		MATCH,
		LPAREN,
		IDENT,
		COLON,
		IDENT,
		RPAREN,
		EOF,
	}

	for _, expected := range expectedTokens {
		yySymType := &yySymType{}
		token := lexer.Lex(yySymType)
		if token != int(expected) {
			t.Errorf("lexer.Lex() = %v, want %v", token, expected)
		}
	}
}
