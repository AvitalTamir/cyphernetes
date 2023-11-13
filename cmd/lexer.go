package cmd

import (
	"fmt"
	"strings"
	"text/scanner"
)

type Token int

const (
	ILLEGAL Token = iota
	WS
)

type Lexer struct {
	s   scanner.Scanner
	buf struct {
		tok Token
		lit string
	}
	afterReturn bool
}

func NewLexer(input string) *Lexer {
	var s scanner.Scanner
	s.Init(strings.NewReader(input))
	s.Whitespace = 1<<'\t' | 1<<'\r' | 1<<' '
	return &Lexer{s: s}
}

func (l *Lexer) Lex(lval *yySymType) int {
	fmt.Println("Lexing...")
	if l.buf.tok == EOF { // If we have already returned EOF, keep returning EOF
		fmt.Println("Zero (buffered EOF)")
		return 0
	}

	// Check if we are capturing a JSONPATH
	if l.afterReturn {
		lval.strVal = ""
		// Consume and ignore any whitespace
		ch := l.s.Peek()
		for ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			l.s.Next() // Consume the whitespace
			ch = l.s.Peek()
		}

		// Capture the JSONPATH
		for isValidJsonPathChar(ch) {
			l.s.Next() // Consume the character
			lval.strVal += string(ch)
			ch = l.s.Peek()
		}

		l.afterReturn = false
		fmt.Println("Returning JSONPATH token with value:", lval.strVal)
		return int(JSONPATH)
	}

	// Handle normal tokens
	tok := l.s.Scan()
	fmt.Println("Scanned token:", tok)

	switch tok {
	case scanner.Ident:
		lit := l.s.TokenText()
		if strings.ToUpper(lit) == "MATCH" {
			fmt.Println("Returning MATCH token")
			return int(MATCH)
		} else if strings.ToUpper(lit) == "RETURN" {
			l.afterReturn = true // Next we'll capture the jsonPath
			l.buf.tok = RETURN   // Indicate that we've read a RETURN.
			fmt.Println("Returning RETURN token")
			return int(RETURN)
		} else {
			lval.strVal = lit
			fmt.Println("Returning IDENT token with value:", lval.strVal)
			return int(IDENT)
		}
	case scanner.EOF:
		fmt.Println("Returning EOF token")
		l.buf.tok = EOF // Indicate that we've read an EOF.
		return int(EOF)
	case '(':
		fmt.Println("Returning LPAREN token")
		return int(LPAREN)
	case ':':
		l.buf.tok = COLON // Indicate that we've read a COLON.
		fmt.Println("Returning COLON token")
		return int(COLON)
	case ')':
		fmt.Println("Returning RPAREN token")
		return int(RPAREN)
	case ' ', '\t', '\r':
		fmt.Println("Ignoring whitespace")
		return int(WS) // Ignore whitespace.
	default:
		fmt.Println("Illegal token:", tok)
		return int(ILLEGAL)
	}
}

// Helper function to check if a character is valid in a jsonPath
func isValidJsonPathChar(tok rune) bool {
	// convert to string for easier comparison
	char := string(tok)

	return char == "." || char == "[" || char == "]" ||
		(char >= "0" && char <= "9") || char == "_" ||
		(char >= "a" && char <= "z") || (char >= "A" && char <= "Z") ||
		char == "\"" || char == "*"
}

func (l *Lexer) Error(e string) {
	fmt.Printf("Error: %v\n", e)
}

type ASTNode struct {
	Name string
	Kind string
}

func NewASTNode(name, kind string) *ASTNode {
	return &ASTNode{Name: name, Kind: kind}
}
