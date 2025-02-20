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
	isInJsonPath  bool
	lastToken     Token
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
	l.skipWhitespace()

	if l.s.Peek() == scanner.EOF {
		return Token{Type: EOF, Literal: ""}
	}

	// Check if we're in a property access context (after a dot in a WHERE or RETURN clause)
	// or if we're in a WHERE clause and the last token was a comma or AND
	if (l.lastToken.Type == DOT || l.lastToken.Type == COMMA || l.lastToken.Type == AND || l.lastToken.Type == WHERE) && !l.inJsonData && !l.inPropertyKey {
		l.isInJsonPath = true
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
			case "NOT":
				return Token{Type: NOT, Literal: lit}
			case "DATETIME":
				return Token{Type: DATETIME, Literal: "datetime"}
			case "DURATION":
				return Token{Type: DURATION, Literal: "duration"}
			default:
				debugLog("Returning IDENT token: '%s'", lit)
				if !l.isInJsonPath {
					return Token{Type: IDENT, Literal: lit}
				}
			}
		}
		var fullLit strings.Builder
		fullLit.WriteString(lit)

		// Handle dots in identifiers
		for l.s.Peek() == '.' || l.s.Peek() == '"' || (l.isInJsonPath && l.s.Peek() == '\\') || l.s.Peek() == '-' {
			// First check for escaped dots in JSON paths
			if l.isInJsonPath && l.s.Peek() == '\\' {
				l.s.Next() // consume backslash
				if l.s.Peek() == '.' {
					l.s.Next()                 // consume dot
					fullLit.WriteString("\\.") // Write the escaped dot

					// Continue scanning the rest of the identifier
					scanTok := l.s.Scan()
					if scanTok != scanner.Ident {
						return Token{Type: ILLEGAL, Literal: fullLit.String()}
					}
					fullLit.WriteString(l.s.TokenText())

					// Continue scanning for more parts (like helm.sh/release-name)
					for l.s.Peek() == '.' || l.s.Peek() == '/' || (l.isInJsonPath && l.s.Peek() == '\\') || l.s.Peek() == '-' {
						if l.isInJsonPath && l.s.Peek() == '\\' {
							l.s.Next() // consume backslash
							if l.s.Peek() == '.' {
								l.s.Next() // consume dot
								fullLit.WriteString("\\.")
								scanTok = l.s.Scan()
								if scanTok != scanner.Ident {
									return Token{Type: ILLEGAL, Literal: fullLit.String()}
								}
								fullLit.WriteString(l.s.TokenText())
								continue
							}
							return Token{Type: ILLEGAL, Literal: "\\"}
						}
						ch := l.s.Next()
						fullLit.WriteRune(ch)
						scanTok = l.s.Scan()
						if scanTok != scanner.Ident {
							return Token{Type: ILLEGAL, Literal: fullLit.String()}
						}
						fullLit.WriteString(l.s.TokenText())
					}
					debugLog("Built escaped identifier: '%s'", fullLit.String())
					resultTok := Token{Type: IDENT, Literal: fullLit.String()}
					l.lastToken = resultTok
					return resultTok
				}
				// Not a dot after backslash, treat as illegal
				return Token{Type: ILLEGAL, Literal: "\\"}
			}

			next := l.s.Next()
			debugLog("Lexer peeked: '%c'", next)
			if next == '"' {
				// Include quotes in the identifier
				fullLit.WriteRune('"')
				debugLog("Added quote to identifier: '%s'", fullLit.String())
				continue
			}

			if next == '-' {
				fullLit.WriteRune('-')
				scanTok := l.s.Scan()
				if scanTok != scanner.Ident {
					return Token{Type: ILLEGAL, Literal: fullLit.String()}
				}
				fullLit.WriteString(l.s.TokenText())
				continue
			}

			// Handle dots differently for resource kinds vs JSON paths
			if l.inNodeLabel {
				// For resource kinds, combine with dots
				fullLit.WriteRune('.')
				scanTok := l.s.Scan()
				if scanTok != scanner.Ident {
					return Token{Type: ILLEGAL, Literal: fullLit.String()}
				}
				fullLit.WriteString(l.s.TokenText())
				continue
			}

			// For JSON paths, return separate tokens
			debugLog("Final identifier: '%s'", fullLit.String())
			l.buf.hasNext = true
			l.buf.tok = DOT
			l.buf.lit = "."
			tok := Token{Type: IDENT, Literal: fullLit.String()}
			l.lastToken = tok
			return tok
		}
		debugLog("Final identifier: '%s'", fullLit.String())
		tok := Token{Type: IDENT, Literal: fullLit.String()}
		l.lastToken = tok
		return tok

	case scanner.Int:
		debugLog("Got number: '%s'", l.s.TokenText())
		tok := Token{Type: NUMBER, Literal: l.s.TokenText()}
		l.lastToken = tok
		return tok

	case scanner.String:
		debugLog("Got string: '%s'", l.s.TokenText())
		lit := l.s.TokenText()
		var tok Token
		if l.inNodeLabel && l.inPropertyKey && !l.inJsonData {
			l.inPropertyKey = false
			tok = Token{Type: IDENT, Literal: strings.Trim(lit, "\"")}
		} else {
			tok = Token{Type: STRING, Literal: lit}
		}
		l.lastToken = tok
		return tok

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
		tok := Token{Type: DOT, Literal: "."}
		l.lastToken = tok
		return tok

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
			return Token{Type: MINUS, Literal: "-"}
		}

	case '+':
		return Token{Type: PLUS, Literal: "+"}

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

// Add method to set JSON path parsing state
func (l *Lexer) SetParsingJsonPath(parsing bool) {
	l.isInJsonPath = parsing
}

func (l *Lexer) skipWhitespace() {
	for {
		ch := l.s.Peek()
		if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
			break
		}
		l.s.Next()
	}
}
