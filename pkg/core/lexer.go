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

	// Loop until a valid token is found or EOF
	for {
		// Skip whitespace first
		l.skipWhitespace()

		// Check for single-line comments (//)
		if l.s.Peek() == '/' {
			ch1 := l.s.Next() // Tentatively consume the first '/'
			if l.s.Peek() == '/' {
				l.s.Next() // Consume the second '/'
				// It's a comment, skip to the end of the line or EOF
				for {
					peekedChar := l.s.Peek() // Peek first
					if peekedChar == '\n' || peekedChar == scanner.EOF {
						// Don't consume EOF here, let the main loop handle it
						if peekedChar == '\n' {
							l.s.Next() // Consume the newline
						}
						break // Break the inner comment skipping loop
					}
					// Consume the character if it's not newline or EOF
					l.s.Next()
				}
				// After skipping the comment (or hitting EOF during skip),
				// continue the main loop to find the *next* token.
				continue // Restarts the outer 'for' loop
			} else {
				// It was just a single '/'. We already consumed it (ch1).
				// Treat single '/' as ILLEGAL.
				return Token{Type: ILLEGAL, Literal: string(ch1)}
			}
		}

		// Check for EOF *after* potentially skipping comments/whitespace
		if l.s.Peek() == scanner.EOF {
			return Token{Type: EOF, Literal: ""}
		}

		// Check if we're in a property access context (after a dot in a WHERE or RETURN clause)
		// or if we're in a WHERE clause and the last token was a comma or AND
		if (l.lastToken.Type == DOT || l.lastToken.Type == COMMA || l.lastToken.Type == AND || l.lastToken.Type == WHERE || l.lastToken.Type == SET) && !l.inJsonData && !l.inPropertyKey {
			l.isInJsonPath = true
		} else {
			// Reset isInJsonPath if not in a relevant context
			l.isInJsonPath = false
		}

		// Scan the next token
		tok := l.s.Scan()

		switch tok {
		case scanner.EOF: // Should be caught by peek earlier, but handle defensively
			return Token{Type: EOF, Literal: ""}

		case scanner.Ident:
			lit := l.s.TokenText()
			debugLog("Lexer got IDENT token: '%s'", lit)
			if !l.inNodeLabel {
				// Handle keywords first
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
				case "ORDER":
					return Token{Type: ORDER, Literal: lit}
				case "BY":
					return Token{Type: BY, Literal: lit}
				case "LIMIT":
					return Token{Type: LIMIT, Literal: lit}
				case "SKIP":
					return Token{Type: SKIP, Literal: lit}
				case "ASC":
					return Token{Type: ASC, Literal: lit}
				case "DESC":
					return Token{Type: DESC, Literal: lit}
				case "DATETIME":
					return Token{Type: DATETIME, Literal: "datetime"}
				case "DURATION":
					return Token{Type: DURATION, Literal: "duration"}
					// If not a keyword, fall through to identifier logic below
				}
			}

			// Check what follows the initial identifier
			peek := l.s.Peek()
			isPotentialSeparator := peek == '.' || peek == '"' || peek == '/' || peek == '-' || (l.isInJsonPath && peek == '\\')

			// Case 1: Simple identifier (not followed by separator) or JSON path segment (followed by '.')
			if !isPotentialSeparator || (peek == '.' && !l.inNodeLabel) {
				// If it's followed by a dot but not in a node label context, treat as path segment.
				// Return the identifier now, the next call will return the DOT.
				// If it's not followed by any separator, it's just a simple identifier.
				debugLog("Returning simple IDENT or path segment: '%s'", lit)
				resultTok := Token{Type: IDENT, Literal: lit}
				l.lastToken = resultTok
				return resultTok
			}

			// Case 2: Complex identifier (resource kind like `deployments.apps` or path with `/ - \.`)
			var fullLit strings.Builder
			fullLit.WriteString(lit)

			for l.s.Peek() == '.' || l.s.Peek() == '"' || l.s.Peek() == '/' || l.s.Peek() == '-' || (l.isInJsonPath && l.s.Peek() == '\\') {
				peek = l.s.Peek() // Re-peek inside the loop

				// Handle JSON path dot separation *within* the loop? No, handled above.
				// If peek is '.' and !l.inNodeLabel, the outer check should have returned the token already.
				// So if we are here and peek is '.', it must be because l.inNodeLabel is true (resource kind).

				// Handle escaped dots in JSON paths
				if l.isInJsonPath && peek == '\\' {
					l.s.Next() // consume backslash
					if l.s.Peek() == '.' {
						l.s.Next()                 // consume dot
						fullLit.WriteString("\\.") // Write the escaped dot
						// Scan the next part of the identifier
						scanTok := l.s.Scan()
						if scanTok != scanner.Ident {
							return Token{Type: ILLEGAL, Literal: fullLit.String()} // Part after escaped dot is not ident
						}
						fullLit.WriteString(l.s.TokenText())
						continue // Continue the complex identifier building loop
					}
					// Not a dot after backslash, treat as illegal
					return Token{Type: ILLEGAL, Literal: fullLit.String() + "\\"}
				}

				// Consume the separator ('.', '"', '/', '-')
				next := l.s.Next()
				fullLit.WriteRune(next) // Append the separator
				debugLog("Added separator '%c' to complex identifier: '%s'", next, fullLit.String())

				// Scan the next part of the identifier
				scanTok := l.s.Scan()
				if scanTok != scanner.Ident {
					// If the char after separator is not IDENT, it's an error
					return Token{Type: ILLEGAL, Literal: fullLit.String()}
				}
				// Append the scanned identifier part
				fullLit.WriteString(l.s.TokenText())
			} // End of complex identifier building loop

			// If the loop finishes, return the fully built complex identifier
			finalLit := fullLit.String()
			debugLog("Final complex identifier: '%s'", finalLit)
			resultTok := Token{Type: IDENT, Literal: finalLit}
			l.lastToken = resultTok
			return resultTok

		case scanner.Int, scanner.Float:
			debugLog("Got number: '%s'", l.s.TokenText())
			resultTok := Token{Type: NUMBER, Literal: l.s.TokenText()}
			l.lastToken = resultTok
			return resultTok

		case scanner.String:
			debugLog("Got string: '%s'", l.s.TokenText())
			lit := l.s.TokenText()
			var resultTok Token
			if l.inNodeLabel && l.inPropertyKey && !l.inJsonData {
				l.inPropertyKey = false
				resultTok = Token{Type: IDENT, Literal: strings.Trim(lit, "\"")}
			} else {
				resultTok = Token{Type: STRING, Literal: lit}
			}
			l.lastToken = resultTok
			return resultTok

		// Handle single characters and operators recognized by Scan()
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
			resultTok := Token{Type: COMMA, Literal: ","}
			l.lastToken = resultTok
			return resultTok
		case '.':
			resultTok := Token{Type: DOT, Literal: "."}
			l.lastToken = resultTok
			return resultTok
		case '=':
			if l.s.Peek() == '~' {
				l.s.Next() // consume '~'
				return Token{Type: REGEX_COMPARE, Literal: "=~"}
			}
			resultTok := Token{Type: EQUALS, Literal: "="}
			l.lastToken = resultTok
			return resultTok
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
				// Context check for dashed names removed as identifier logic handles it
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
				// Disallow <<
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
		case '*': // Handle '*' often used in RETURN or array index
			// Check context? For now, assume it's part of an expression/path
			return Token{Type: ILLEGAL, Literal: "*"} // Treat as ILLEGAL if scanned standalone, specific parsing handles it
		default:
			// If scanner.Scan() returns a rune not handled above, treat it as ILLEGAL.
			// Note: Single '/' is handled by the comment check logic earlier.
			return Token{Type: ILLEGAL, Literal: l.s.TokenText()}
		}
	} // End of the for loop
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
