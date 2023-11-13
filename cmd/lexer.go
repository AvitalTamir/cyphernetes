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

	tok := l.s.Scan()
	fmt.Println("Scanned token:", tok)

	if tok == scanner.EOF {
		l.buf.tok = EOF
		fmt.Println("Returning EOF token")
		return int(EOF)
	}

	if l.buf.tok == COLON && tok == scanner.Ident {
		lval.strVal = l.s.TokenText()
		l.buf.tok = ILLEGAL // Clear the buffer state.
		fmt.Println("Returning IDENT token with value:", lval.strVal)
		return int(IDENT)
	}
	switch tok {
	case scanner.Ident:
		lit := l.s.TokenText()
		// Check if the last token was a COLON to assign the string to the correct field.
		if l.buf.tok == COLON {
			lval.strVal = lit
			l.buf.tok = ILLEGAL // Clear the buffer token.
			fmt.Println("Returning IDENT token with value:", lval.strVal)
			return int(IDENT)
		}
		if strings.ToUpper(lit) == "MATCH" {
			fmt.Println("Returning MATCH token")
			return int(MATCH) // Return MATCH as a special token.
		}
		lval.strVal = lit
		fmt.Println("Returning IDENT token with value:", lval.strVal)
		return int(IDENT)
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
