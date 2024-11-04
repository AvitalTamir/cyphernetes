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
		{
			name:  "MATCH and RETURN with props",
			input: "MATCH (k:Kind {name: \"test\"}) RETURN k.name, k.age",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				LBRACE,
				JSONPATH,
				COLON,
				STRING,
				RBRACE,
				RPAREN,
				RETURN,
				JSONPATH,
				COMMA,
				JSONPATH,
				EOF,
			},
			wantLiterals: []string{
				"",         // MATCH
				"",         // LPAREN
				"k",        // IDENT
				"",         // COLON
				"Kind",     // IDENT
				"",         // LBRACE
				"name",     // IDENT
				"",         // COLON
				"\"test\"", // STRING
				"",         // RBRACE
				"",         // RPAREN
				"",         // RETURN
				"k.name",   // JSONPATH
				"",         // COMMA
				"k.age",    // JSONPATH
				"",         // EOF
			},
		},
		{
			name:  "MATCH and RETURN with props and relationship",
			input: "MATCH (k:Kind {name: \"test\"})-[r:REL]->(k2:Kind) RETURN k.name, k.age",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				LBRACE,
				JSONPATH,
				COLON,
				STRING,
				RBRACE,
				RPAREN,
				REL_BEGINPROPS_NONE,
				IDENT,
				COLON,
				IDENT,
				REL_ENDPROPS_RIGHT,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				RETURN,
				JSONPATH,
				COMMA,
				JSONPATH,
				EOF,
			},
			wantLiterals: []string{
				"",         // MATCH
				"",         // LPAREN
				"k",        // IDENT
				"",         // COLON
				"Kind",     // IDENT
				"",         // LBRACE
				"name",     // IDENT
				"",         // COLON
				"\"test\"", // STRING
				"",         // RBRACE
				"",         // RPAREN
				"",         // REL_BEGINPROPS_LEFT
				"r",        // IDENT
				"",         // COLON
				"REL",      // IDENT
				"",         // REL_ENDPROPS_RIGHT
				"",         // LPAREN
				"k2",       // IDENT
				"",         // COLON
				"Kind",     // IDENT
				"",         // RPAREN
				"",         // RETURN
				"k.name",   // JSONPATH
				"",         // COMMA
				"k.age",    // JSONPATH
				"",         // EOF
			},
		},
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
		// TEST MATCH and CREATE
		{
			name:  "MATCH and CREATE",
			input: "MATCH (k:Kind) CREATE (k2:Kind)",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				CREATE,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				EOF,
			},
			wantLiterals: []string{
				"",     // MATCH
				"",     // LPAREN
				"k",    // IDENT
				"",     // COLON
				"Kind", // IDENT
				"",     // RPAREN
				"",     // CREATE
				"",     // LPAREN
				"k2",   // IDENT
				"",     // COLON
				"Kind", // IDENT
				"",     // RPAREN
				"",     // EOF
			},
		},
		// TEST MATCH WHERE RETURN
		{
			name:  "MATCH WHERE RETURN",
			input: "MATCH (k:Kind) WHERE k.name = \"test\" RETURN k.name",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				WHERE,
				JSONPATH,
				EQUALS,
				STRING,
				RETURN,
				JSONPATH,
				EOF,
			},
			wantLiterals: []string{
				"",         // MATCH
				"",         // LPAREN
				"k",        // IDENT
				"",         // COLON
				"Kind",     // IDENT
				"",         // RPAREN
				"",         // WHERE
				"k.name",   // JSONPATH
				"",         // EQUALS
				"\"test\"", // STRING
				"",         // RETURN
				"k.name",   // JSONPATH
				"",         // EOF
			},
		},
		{
			name:  "MATCH RETURN with COUNT",
			input: "MATCH (k:Kind) RETURN COUNT{k.name} AS name",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				RETURN,
				COUNT,
				LBRACE,
				JSONPATH,
				RBRACE,
				AS,
				IDENT,
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
				"",       // COUNT
				"",       // LBRACE
				"k.name", // JSONPATH
				"",       // RBRACE
				"",       // AS
				"name",   // IDENT
				"",       // EOF
			},
		},
		{
			name:  "MATCH RETURN with COUNT, SUM and JSONPATH",
			input: "MATCH (k:Kind) RETURN k.test, COUNT{k.name} AS name, SUM{k.age} AS age, k.name",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				RETURN,
				JSONPATH,
				COMMA,
				COUNT,
				LBRACE,
				JSONPATH,
				RBRACE,
				AS,
				IDENT,
				COMMA,
				SUM,
				LBRACE,
				JSONPATH,
				RBRACE,
				AS,
				IDENT,
				COMMA,
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
				"k.test", // IDENT
				"",       // COMMA
				"",       // COUNT
				"",       // LBRACE
				"k.name", // JSONPATH
				"",       // RBRACE
				"",       // AS
				"name",   // IDENT
				"",       // COMMA
				"",       // SUM
				"",       // LBRACE
				"k.age",  // JSONPATH
				"",       // RBRACE
				"",       // AS
				"age",    // IDENT
				"",       // COMMA
				"k.name", // JSONPATH
				"",       // EOF
			},
		},
		{
			name:  "MATCH Relationship WHERE with DELETE",
			input: "MATCH (k:Kind)->(s:Kind2) WHERE k.name = \"test\" DELETE k,s",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				REL_NOPROPS_RIGHT,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				WHERE,
				JSONPATH,
				EQUALS,
				STRING,
				DELETE,
				IDENT,
				COMMA,
				IDENT,
				EOF,
			},
			wantLiterals: []string{
				"",         // MATCH
				"",         // LPAREN
				"k",        // IDENT
				"",         // COLON
				"Kind",     // IDENT
				"",         // RPAREN
				"",         // REL_NOPROPS_RIGHT
				"",         // LPAREN
				"s",        // IDENT
				"",         // COLON
				"Kind2",    // IDENT
				"",         // RPAREN
				"",         // WHERE
				"k.name",   // JSONPATH
				"",         // EQUALS
				"\"test\"", // STRING
				"",         // DELETE
				"k",        // IDENT
				"",         // COMMA
				"s",        // IDENT
				"",         // EOF
			},
		},
		{
			name:  "MATCH multiple Relationship WHERE with multiple DELETE",
			input: "MATCH (p:pod)->(rs:replicaSet)->(d:Deployment)->(s:service)->(i:ing) WHERE d.metadata.name = \"test\" DELETE s,i",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				REL_NOPROPS_RIGHT,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				REL_NOPROPS_RIGHT,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				REL_NOPROPS_RIGHT,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				REL_NOPROPS_RIGHT,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				WHERE,
				JSONPATH,
				EQUALS,
				STRING,
				DELETE,
				IDENT,
				COMMA,
				IDENT,
				EOF,
			},
			wantLiterals: []string{
				"",                // MATCH
				"",                // LPAREN
				"p",               // IDENT
				"",                // COLON
				"pod",             // IDENT
				"",                // RPAREN
				"",                // REL_NOPROPS_RIGHT
				"",                // LPAREN
				"rs",              // IDENT
				"",                // COLON
				"replicaSet",      // IDENT
				"",                // RPAREN
				"",                // REL_NOPROPS_RIGHT
				"",                // LPAREN
				"d",               // IDENT
				"",                // COLON
				"Deployment",      // IDENT
				"",                // RPAREN
				"",                // REL_NOPROPS_RIGHT
				"",                // LPAREN
				"s",               // IDENT
				"",                // COLON
				"service",         // IDENT
				"",                // RPAREN
				"",                // REL_NOPROPS_RIGHT
				"",                // LPAREN
				"i",               // IDENT
				"",                // COLON
				"ing",             // IDENT
				"",                // RPAREN
				"",                // WHERE
				"d.metadata.name", // JSONPATH
				"",                // EQUALS
				"\"test\"",        // STRING
				"",                // DELETE
				"s",               // IDENT
				"",                // COMMA
				"i",               // IDENT
				"",                // EOF
			},
		},
		{
			name:  "MATCH WHERE CONTAINS",
			input: "MATCH (k:Kind) WHERE k.name CONTAINS \"^test.*\" RETURN k.name",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				WHERE,
				JSONPATH,
				CONTAINS,
				STRING,
				RETURN,
				JSONPATH,
				EOF,
			},
			wantLiterals: []string{
				"",            // MATCH
				"",            // LPAREN
				"k",           // IDENT
				"",            // COLON
				"Kind",        // IDENT
				"",            // RPAREN
				"",            // WHERE
				"k.name",      // JSONPATH
				"",            // CONTAINS
				"\"^test.*\"", // STRING
				"",            // RETURN
				"k.name",      // JSONPATH
				"",            // EOF
			},
		},
		{
			name:  "MATCH WHERE REGEX_COMPARE",
			input: "MATCH (k:Kind) WHERE k.name =~ \"^test.*\" RETURN k.name",
			wantTokens: []int{
				MATCH,
				LPAREN,
				IDENT,
				COLON,
				IDENT,
				RPAREN,
				WHERE,
				JSONPATH,
				REGEX_COMPARE,
				STRING,
				RETURN,
				JSONPATH,
				EOF,
			},
			wantLiterals: []string{
				"",            // MATCH
				"",            // LPAREN
				"k",           // IDENT
				"",            // COLON
				"Kind",        // IDENT
				"",            // RPAREN
				"",            // WHERE
				"k.name",      // JSONPATH
				"",            // REGEX_COMPARE
				"\"^test.*\"", // STRING
				"",            // RETURN
				"k.name",      // JSONPATH
				"",            // EOF
			},
		},
		{
			name:  "IN MATCH RETURN",
			input: "IN staging MATCH (k:Kind) RETURN k.name",
			wantTokens: []int{
				IN,
				IDENT,
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
				"",        // IN
				"staging", // IDENT
				"",        // MATCH
				"",        // LPAREN
				"k",       // IDENT
				"",        // COLON
				"Kind",    // IDENT
				"",        // RPAREN
				"",        // RETURN
				"k.name",  // JSONPATH
				"",        // EOF
			},
		},
		{
			name:  "IN multiple contexts MATCH RETURN",
			input: "IN staging, production MATCH (k:Kind) RETURN k.name",
			wantTokens: []int{
				IN,
				IDENT,
				COMMA,
				IDENT,
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
				"",           // IN
				"staging",    // IDENT
				"",           // COMMA
				"production", // IDENT
				"",           // MATCH
				"",           // LPAREN
				"k",          // IDENT
				"",           // COLON
				"Kind",       // IDENT
				"",           // RPAREN
				"",           // RETURN
				"k.name",     // JSONPATH
				"",           // EOF
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
