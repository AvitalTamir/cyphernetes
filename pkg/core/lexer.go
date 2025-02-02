package core

import (
	"strings"
	"text/scanner"
)

type Lexer struct {
	s   scanner.Scanner
	buf struct {
		tok     TokenType
		lit     string
		hasNext bool
	}
	inContexts    bool
	inNodeLabel   bool
	inPropertyKey bool
	inJsonData    bool
}

func NewLexer(input string) *Lexer {
	var s scanner.Scanner
	s.Init(strings.NewReader(input))
	s.Whitespace = 1<<'\t' | 1<<'\r' | 1<<' '

	return &Lexer{s: s}
}

// NextToken returns the next token in the input
func (l *Lexer) NextToken() Token {
	// If we have a buffered token, return it
	if l.buf.hasNext {
		l.buf.hasNext = false
		return Token{Type: l.buf.tok, Literal: l.buf.lit}
	}

	// Skip whitespace
	for {
		ch := l.s.Peek()
		if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
			break
		}
		l.s.Next()
	}

	tok := l.s.Scan()

	switch tok {
	case scanner.EOF:
		return Token{Type: EOF, Literal: ""}

	case scanner.Ident:
		lit := l.s.TokenText()
		debugLog("Lexer got IDENT token: '%s'", lit)
		if !l.inNodeLabel {
			switch strings.ToUpper(lit) {
			case "MATCH":
				return Token{Type: MATCH, Literal: lit}
			case "CREATE":
				return Token{Type: CREATE, Literal: lit}
			case "WHERE":
				return Token{Type: WHERE, Literal: lit}
			case "SET":
				return Token{Type: SET, Literal: lit}
			case "DELETE":
				return Token{Type: DELETE, Literal: lit}
			case "RETURN":
				return Token{Type: RETURN, Literal: lit}
			case "IN":
				return Token{Type: IN, Literal: lit}
			case "AS":
				return Token{Type: AS, Literal: lit}
			case "COUNT":
				return Token{Type: COUNT, Literal: lit}
			case "SUM":
				return Token{Type: SUM, Literal: lit}
			case "CONTAINS":
				return Token{Type: CONTAINS, Literal: lit}
			case "AND":
				return Token{Type: AND, Literal: lit}
			case "TRUE", "FALSE":
				return Token{Type: BOOLEAN, Literal: lit}
			case "NULL":
				return Token{Type: NULL, Literal: lit}
			default:
				debugLog("Returning IDENT token: '%s'", lit)
				return Token{Type: IDENT, Literal: lit}
			}
		}
		var fullLit strings.Builder
		fullLit.WriteString(lit)

		// Handle dots in identifiers
		for l.s.Peek() == '.' || l.s.Peek() == '"' {
			next := l.s.Next()
			debugLog("Lexer peeked: '%c'", next)
			if next == '"' {
				// Include quotes in the identifier
				fullLit.WriteRune('"')
				debugLog("Added quote to identifier: '%s'", fullLit.String())
				continue
			}
			fullLit.WriteRune('.')

			tok := l.s.Scan()
			if tok != scanner.Ident {
				return Token{Type: ILLEGAL, Literal: fullLit.String()}
			}
			fullLit.WriteString(l.s.TokenText())
			debugLog("Built identifier: '%s'", fullLit.String())
		}
		debugLog("Final identifier: '%s'", fullLit.String())
		return Token{Type: IDENT, Literal: fullLit.String()}

	case scanner.Int:
		return Token{Type: NUMBER, Literal: l.s.TokenText()}

	case scanner.String:
		lit := l.s.TokenText()
		if l.inNodeLabel && l.inPropertyKey && !l.inJsonData {
			l.inPropertyKey = false
			return Token{Type: IDENT, Literal: strings.Trim(lit, "\"")}
		}
		return Token{Type: STRING, Literal: lit}

	case '*':
		return Token{Type: ILLEGAL, Literal: "*"}

	case '[':
		return Token{Type: LBRACKET, Literal: "["}
	case ']':
		if l.s.Peek() == '-' {
			l.s.Next()
			if l.s.Peek() == '>' {
				l.s.Next()
				return Token{Type: REL_ENDPROPS_RIGHT, Literal: "]->"}
			}
			return Token{Type: REL_ENDPROPS_NONE, Literal: "]-"}
		}
		return Token{Type: RBRACKET, Literal: "]"}
	case '(':
		return Token{Type: LPAREN, Literal: "("}
	case ')':
		l.inNodeLabel = false
		return Token{Type: RPAREN, Literal: ")"}
	case '{':
		if !l.inNodeLabel && !l.inPropertyKey {
			l.inJsonData = true
		}
		l.inPropertyKey = true
		return Token{Type: LBRACE, Literal: "{"}
	case '}':
		l.inJsonData = false
		return Token{Type: RBRACE, Literal: "}"}
	case ':':
		if !l.inNodeLabel {
			l.inNodeLabel = true
		}
		l.inPropertyKey = false
		return Token{Type: COLON, Literal: ":"}
	case ',':
		if !l.inJsonData {
			l.inPropertyKey = true
		}
		return Token{Type: COMMA, Literal: ","}
	case '.':
		return Token{Type: DOT, Literal: "."}

	case '=':
		if l.s.Peek() == '~' {
			l.s.Next() // consume '~'
			return Token{Type: REGEX_COMPARE, Literal: "=~"}
		}
		return Token{Type: EQUALS, Literal: "="}

	case '!':
		if l.s.Peek() == '=' {
			l.s.Next() // consume '='
			return Token{Type: NOT_EQUALS, Literal: "!="}
		}
		return Token{Type: ILLEGAL, Literal: "!"}

	case '-':
		switch l.s.Peek() {
		case '>':
			l.s.Next()
			return Token{Type: REL_NOPROPS_RIGHT, Literal: "->"}
		case '[':
			l.s.Next()
			return Token{Type: REL_BEGINPROPS_NONE, Literal: "-["}
		case '-':
			l.s.Next()
			return Token{Type: REL_NOPROPS_NONE, Literal: "--"}
		default:
			if l.inContexts {
				return Token{Type: IDENT, Literal: "-"}
			}
			return Token{Type: ILLEGAL, Literal: "-"}
		}

	case '<':
		switch l.s.Peek() {
		case '-':
			l.s.Next()
			if l.s.Peek() == '[' {
				l.s.Next()
				return Token{Type: REL_BEGINPROPS_LEFT, Literal: "<-["}
			}
			return Token{Type: REL_NOPROPS_LEFT, Literal: "<-"}
		case '=':
			l.s.Next()
			return Token{Type: LESS_THAN_EQUALS, Literal: "<="}
		case '<':
			l.s.Next()
			return Token{Type: ILLEGAL, Literal: "<<"}
		default:
			return Token{Type: LESS_THAN, Literal: "<"}
		}

	case '>':
		if l.s.Peek() == '=' {
			l.s.Next()
			return Token{Type: GREATER_THAN_EQUALS, Literal: ">="}
		}
		return Token{Type: GREATER_THAN, Literal: ">"}
	}

	return Token{Type: ILLEGAL, Literal: l.s.TokenText()}
}

// Add Peek method to Lexer
func (l *Lexer) Peek() rune {
	return l.s.Peek()
}

// Add method to set context parsing state
func (l *Lexer) SetParsingContexts(parsing bool) {
	l.inContexts = parsing
}

// Add method to set JSON data parsing state
func (l *Lexer) SetParsingJsonData(parsing bool) {
	l.inJsonData = true
}
