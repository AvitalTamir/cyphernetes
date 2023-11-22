package parser

import (
	"reflect"
	"testing"
)

// Define a function to simulate the yacc parsing process
func simulateLexing(input string) ([]int, []string, error) {
	lexer := NewLexer(input)
	var tokens []int
	var literals []string

	for {
		var lval yySymType
		token := lexer.Lex(&lval)
		tokens = append(tokens, token)
		literals = append(literals, lval.strVal)

		if token == int(EOF) || token == int(ILLEGAL) {
			break
		}
	}

	return tokens, literals, nil
}

func TestLexer(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantTokens   []int
		wantLiterals []string
	}{
		{
			name:  "MATCH and RETURN",
			input: "MATCH (k:Kind) RETURN k.name",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				RETURN,
				JSONPATH,
				EOF,
			},
			wantLiterals: []string{
				"",       // MATCH
				"",       // LPAREN
				"k",      // IDENT
				"",       // COLON
				"Kind",   // IDENT
				"",       // RPAREN
				"",       // RETURN
				"k.name", // JSONPATH
				"",       // EOF
			},
		},
		// Add more test cases as needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTokens, gotLiterals, err := simulateLexing(tt.input)
			if err != nil {
				t.Errorf("simulateLexing() error = %v", err)
				return
			}
			if !reflect.DeepEqual(gotTokens, tt.wantTokens) {
				t.Errorf("simulateLexing() gotTokens = %v, want %v", gotTokens, tt.wantTokens)
			}
			if !reflect.DeepEqual(gotLiterals, tt.wantLiterals) {
				t.Errorf("simulateLexing() gotLiterals = %v, want %v", gotLiterals, tt.wantLiterals)
			}
		})
	}
}
