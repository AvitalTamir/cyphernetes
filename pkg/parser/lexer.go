package parser

import (
	"log"
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
	definingProps  bool
	definingReturn bool
	definingSet    bool
	definingCreate bool
	definingMatch  bool
	definingWhere  bool
}

func NewLexer(input string) *Lexer {
	var s scanner.Scanner
	s.Init(strings.NewReader(input))
	s.Whitespace = 1<<'\t' | 1<<'\r' | 1<<' '
	return &Lexer{s: s}
}

func (l *Lexer) Lex(lval *yySymType) int {
	debugLog("Lexing... ", l.s.Peek(), " (", string(l.s.Peek()), ")")
	if l.buf.tok == EOF { // If we have already returned EOF, keep returning EOF
		logDebug("Zero (buffered EOF)")
		return 0
	}

	// Check if we are capturing a JSONPATH
	if l.buf.tok == RETURN || l.buf.tok == SET || l.buf.tok == WHERE ||
		l.buf.tok == LBRACE && l.definingMatch || (l.buf.tok == COMMA && l.definingProps) ||
		l.buf.tok == COMMA && l.definingReturn || l.buf.tok == COMMA && l.definingSet || l.buf.tok == COMMA && l.definingWhere {
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

		l.buf.tok = ILLEGAL // Indicate that we've read a JSONPATH.
		logDebug("Returning JSONPATH token with value:", lval.strVal)
		return int(JSONPATH)
		// Check if we are capturing a JSONDATA
	} else if l.buf.tok == LBRACE && l.definingCreate {
		// add a first '{', consume the string until we find a ')', and return JSONDATA token with the string as value
		lval.strVal = "{"
		// Consume and ignore any whitespace
		ch := l.s.Peek()
		// Capture the JSONDATA
		for ch != ')' {
			l.s.Next() // Consume the character
			// ignore whitespace
			if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
				lval.strVal += string(ch)
			}
			ch = l.s.Peek()
		}

		l.buf.tok = ILLEGAL // Indicate that we've read a JSONDATA.
		l.definingCreate = false
		logDebug("Returning JSONDATA token with value:", lval.strVal)
		return int(JSONDATA)
	}

	// Handle normal tokens
	tok := l.s.Scan()
	logDebug("Scanned token:", tok)
	logDebug("Token text:", string(tok))

	switch tok {
	case scanner.Ident:
		lit := l.s.TokenText()
		if strings.ToUpper(lit) == "MATCH" {
			logDebug("Returning MATCH token")
			l.buf.tok = MATCH // Indicate that we've read a MATCH.
			l.definingMatch = true
			l.definingSet = false
			l.definingCreate = false
			l.definingReturn = false
			l.definingWhere = false
			return int(MATCH)
		} else if strings.ToUpper(lit) == "SET" {
			l.buf.tok = SET // Indicate that we've read a SET.
			l.definingSet = true
			l.definingCreate = false
			l.definingMatch = false
			l.definingReturn = false
			l.definingWhere = false
			logDebug("Returning SET token")
			return int(SET)
		} else if strings.ToUpper(lit) == "DELETE" {
			logDebug("Returning SET token")
			return int(DELETE)
		} else if strings.ToUpper(lit) == "CREATE" {
			logDebug("Returning CREATE token")
			l.definingCreate = true
			l.definingSet = false
			l.definingMatch = false
			l.definingReturn = false
			l.definingWhere = false
			return int(CREATE)
		} else if strings.ToUpper(lit) == "RETURN" {
			l.buf.tok = RETURN // Indicate that we've read a RETURN.
			l.definingReturn = true
			l.definingSet = false
			l.definingCreate = false
			l.definingMatch = false
			l.definingWhere = false
			logDebug("Returning RETURN token")
			return int(RETURN)
		} else if strings.ToUpper(lit) == "WHERE" {
			logDebug("Returning WHERE token")
			l.definingWhere = true
			l.definingReturn = false
			l.definingSet = false
			l.definingCreate = false
			l.definingMatch = false
			l.buf.tok = WHERE // Indicate that we've read a WHERE.
			return int(WHERE)
		} else if strings.ToUpper(lit) == "TRUE" || strings.ToUpper(lit) == "FALSE" {
			lval.strVal = l.s.TokenText()
			logDebug("Returning BOOLEAN token with value:", lval.strVal)
			return int(BOOLEAN)
		} else {
			lval.strVal = lit
			logDebug("Returning IDENT token with value:", lval.strVal)
			return int(IDENT)
		}
	case scanner.EOF:
		logDebug("Returning EOF token")
		l.definingReturn = false // End of the RETURN clause
		l.buf.tok = EOF          // Indicate that we've read an EOF.
		return int(EOF)
	case '(':
		l.definingProps = true // Indicate that we've read a COLON.
		logDebug("Returning LPAREN token")
		l.buf.tok = LPAREN // Indicate that we've read a LPAREN.
		return int(LPAREN)
	case ':':
		logDebug("Returning COLON token")
		return int(COLON)
	case '=':
		logDebug("Returning EQUALS token")
		return int(EQUALS)
	case ')':
		logDebug("Returning RPAREN token")
		l.definingProps = false // Indicate that we've read a RPAREN.
		return int(RPAREN)
	case ' ', '\t', '\r':
		logDebug("Ignoring whitespace")
		return int(WS) // Ignore whitespace.
	case '{':
		// Capture a JSON object
		l.buf.tok = LBRACE // Indicate that we've read a LBRACE.
		logDebug("Returning LBRACE token")
		return int(LBRACE)
	case '}':
		logDebug("Returning RBRACE token")
		return int(RBRACE)
	case -6: // QUOTE
		lval.strVal = l.s.TokenText()
		logDebug("Returning STRING token with value:", lval.strVal)
		return int(STRING)
	case scanner.Int:
		lval.strVal = l.s.TokenText()
		logDebug("Returning INT token with value:", lval.strVal)
		return int(INT)
	case ',':
		logDebug("Returning COMMA token")
		l.buf.tok = COMMA // Indicate that we've read a COMMA.
		return int(COMMA)
	case '-':
		ch := l.s.Peek()
		if ch == 62 {
			l.s.Next() // Consume '>'
			return int(REL_NOPROPS_RIGHT)
		} else if ch == '[' {
			l.s.Next() // Consume '['
			return int(REL_BEGINPROPS_NONE)
		} else if ch == '-' {
			l.s.Next() // Consume '-'
			return int(REL_NOPROPS_NONE)
		} else {
			return int(ILLEGAL)
		}
	case '<':
		ch := l.s.Peek()
		if ch == '-' {
			l.s.Next() // Consume '-'
			ch = l.s.Peek()
			if ch == '[' {
				l.s.Next() // Consume '['
				return int(REL_BEGINPROPS_LEFT)
			} else if ch == '(' {
				return int(REL_NOPROPS_LEFT)
			} else {
				return int(ILLEGAL)
			}
		}
		return int(ILLEGAL)
	case ']':
		ch := l.s.Peek()
		if ch == '-' {
			l.s.Next() // Consume '-'
			ch = l.s.Peek()
			if ch == '>' {
				l.s.Next() // Consume '>'
				return int(REL_ENDPROPS_RIGHT)
			} else if ch == '(' {
				return int(REL_ENDPROPS_NONE)
			} else {
				return int(ILLEGAL)
			}
		}
		return int(ILLEGAL)
	case '>', '[':
		return int(ILLEGAL)
	default:
		logDebug("Illegal token:", tok)
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
		char == "\"" || char == "*" || char == "$" || char == "#" ||
		char == "/" || char == "-"
}

func (l *Lexer) Error(e string) {
	log.Printf("Error: %v\n", e)
}

type ASTNode struct {
	Name string
	Kind string
}

func NewASTNode(name, kind string) *ASTNode {
	return &ASTNode{Name: name, Kind: kind}
}
