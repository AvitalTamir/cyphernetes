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
		// {
		// 	name:  "MATCH and RETURN with props",
		// 	input: "MATCH (k:Kind {name: \"test\"}) RETURN k.name, k.age",
		// 	wantTokens: []int{
		// 		MATCH,
		// 		LPAREN,
		// 		IDENT,
		// 		COLON,
		// 		IDENT,
		// 		LBRACE,
		// 		IDENT,
		// 		COLON,
		// 		STRING,
		// 		RBRACE,
		// 		RPAREN,
		// 		RETURN,
		// 		JSONPATH,
		// 		COMMA,
		// 		JSONPATH,
		// 		EOF,
		// 	},
		// 	wantLiterals: []string{
		// 		"",         // MATCH
		// 		"",         // LPAREN
		// 		"k",        // IDENT
		// 		"",         // COLON
		// 		"Kind",     // IDENT
		// 		"",         // LBRACE
		// 		"name",     // IDENT
		// 		"",         // COLON
		// 		"\"test\"", // STRING
		// 		"",         // RBRACE
		// 		"",         // RPAREN
		// 		"",         // RETURN
		// 		"k.name",   // JSONPATH
		// 		"",         // COMMA
		// 		"k.age",    // JSONPATH
		// 		"",         // EOF
		// 	},
		// },
		// {
		// 	name:  "MATCH and RETURN with props and relationship",
		// 	input: "MATCH (k:Kind {name: \"test\"})-[r:REL]->(k2:Kind) RETURN k.name, k.age",
		// 	wantTokens: []int{
		// 		MATCH,
		// 		LPAREN,
		// 		IDENT,
		// 		COLON,
		// 		IDENT,
		// 		LBRACE,
		// 		IDENT,
		// 		COLON,
		// 		STRING,
		// 		RBRACE,
		// 		REL_NOPROPS_RIGHT,
		// 		LPAREN,
		// 		IDENT,
		// 		COLON,
		// 		IDENT,
		// 		RPAREN,
		// 		RETURN,
		// 		JSONPATH,
		// 		COMMA,
		// 		JSONPATH,
		// 		EOF,
		// 	},
		// 	wantLiterals: []string{
		// 		"",         // MATCH
		// 		"",         // LPAREN
		// 		"k",        // IDENT
		// 		"",         // COLON
		// 		"Kind",     // IDENT
		// 		"",         // LBRACE
		// 		"name",     // IDENT
		// 		"",         // COLON
		// 		"\"test\"", // STRING
		// 		"",         // RBRACE
		// 		"",         // RPAREN
		// 		"",         // REL_NOPROPS_RIGHT
		// 		"",         // LPAREN
		// 		"k2",       // IDENT
		// 		"",         // COLON
		// 		"Kind",     // IDENT
		// 		"",         // RPAREN
		// 		"",         // RETURN
		// 		"k.name",   // JSONPATH
		// 		"",         // COMMA
		// 		"k.age",    // JSONPATH
		// 		"",         // EOF
		// 	},
		// },
		// Add a test for MATCH and SET
		{
			name:  "MATCH and SET",
			input: "MATCH (k:Kind) SET k.name = \"test\"",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				SET,
				JSONPATH,
				EQUALS,
				STRING,
				EOF,
			},
			wantLiterals: []string{
				"",         // MATCH
				"",         // LPAREN
				"k",        // IDENT
				"",         // COLON
				"Kind",     // IDENT
				"",         // RPAREN
				"",         // SET
				"k.name",   // JSONPATH
				"",         // EQUALS
				"\"test\"", // STRING
				"",         // EOF
			},
		},
		// Add a test for MATCH and DELETE
		{
			name:  "MATCH and DELETE",
			input: "MATCH (k:Kind) DELETE k",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				DELETE,
				IDENT,
				EOF,
			},
			wantLiterals: []string{
				"",     // MATCH
				"",     // LPAREN
				"k",    // IDENT
				"",     // COLON
				"Kind", // IDENT
				"",     // RPAREN
				"",     // DELETE
				"k",    // IDENT
				"",     // EOF
			},
		},
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
