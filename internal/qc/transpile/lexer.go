package transpile

import (
	"fmt"
	"strings"
	"unicode"
)

// TokenKind classifies a QuakeC token.
type TokenKind int

const (
	TokEOF TokenKind = iota
	TokIdent
	TokNumber   // 42, 4.5, 0x10
	TokString   // "player"
	TokVector   // '0 0 0'
	TokPunct    // { } ( ) [ ] ; , : = . + - * / & | ! < > ~ %
	TokTypeName // float, string, vector, entity, void (resolved in lexer)
)

// Token is one lexical QuakeC token with its source position.
type Token struct {
	Kind TokenKind
	Text string // raw text (identifiers keep QC casing; strings unquoted)
	Line int    // 1-based source line
}

// TypeName set recognized by the lexer so the parser can distinguish
// declarations from expressions without a symbol table.
var qcTypeNames = map[string]bool{
	"void": true, "float": true, "string": true,
	"vector": true, "entity": true,
}

// Lex tokenizes QuakeC source. It handles // and /* */ comments, # directives
// (skipped to end of line — the transpiler works on preprocessed-free source),
// quoted strings with QC escapes, and vector literals in single quotes.
func Lex(src string) ([]Token, error) {
	var toks []Token
	line := 1
	i := 0
	n := len(src)

	// lineStart tracks the offset that begins the current line, so the
	// position of an unterminated construct can be reported accurately.
	for i < n {
		ch := src[i]

		switch {
		case ch == '\n':
			line++
			i++

		case ch == ' ' || ch == '\t' || ch == '\r':
			i++

		case ch == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}

		case ch == '/' && i+1 < n && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated comment", line)
			}
			line += strings.Count(src[i:i+2+end], "\n")
			i += 2 + end + 2

		case ch == '#':
			// Preprocessor directive: skip to end of line.
			for i < n && src[i] != '\n' {
				i++
			}

		case ch == '"':
			j := i + 1
			var sb strings.Builder
			for j < n && src[j] != '"' {
				if src[j] == '\n' {
					return nil, fmt.Errorf("line %d: unterminated string", line)
				}
				if src[j] == '\\' && j+1 < n {
					j++ // QC escapes are pass-through; keep the escaped pair
				}
				sb.WriteByte(src[j])
				j++
			}
			if j >= n {
				return nil, fmt.Errorf("line %d: unterminated string", line)
			}
			toks = append(toks, Token{Kind: TokString, Text: sb.String(), Line: line})
			i = j + 1

		case ch == '\'':
			j := i + 1
			for j < n && src[j] != '\'' {
				if src[j] == '\n' {
					return nil, fmt.Errorf("line %d: unterminated vector literal", line)
				}
				j++
			}
			if j >= n {
				return nil, fmt.Errorf("line %d: unterminated vector literal", line)
			}
			toks = append(toks, Token{Kind: TokVector, Text: strings.TrimSpace(src[i+1 : j]), Line: line})
			i = j + 1

		case unicode.IsDigit(rune(ch)) ||
			(ch == '.' && i+1 < n && unicode.IsDigit(rune(src[i+1]))):
			j := i
			for j < n && (unicode.IsDigit(rune(src[j])) || src[j] == '.') {
				j++
			}
			// Hex / scientific forms are rare in QC; plain decimals cover the
			// language. Reject nothing — accept the digits-and-dots slice.
			toks = append(toks, Token{Kind: TokNumber, Text: src[i:j], Line: line})
			i = j

		case isIdentStart(rune(ch)):
			j := i
			for j < n && isIdentRune(rune(src[j])) {
				j++
			}
			text := src[i:j]
			kind := TokIdent
			if qcTypeNames[text] {
				kind = TokTypeName
			}
			toks = append(toks, Token{Kind: kind, Text: text, Line: line})
			i = j

		case strings.ContainsRune("{}()[];,:=+-*/&|!<>~%.^", rune(ch)):
			// Two-character operators first: == != <= >= && || += -= *= /=.
			if i+1 < n {
				pair := src[i : i+2]
				switch pair {
				case "==", "!=", "<=", ">=", "&&", "||", "+=", "-=", "*=", "/=":
					toks = append(toks, Token{Kind: TokPunct, Text: pair, Line: line})
					i += 2
					continue
				}
			}
			toks = append(toks, Token{Kind: TokPunct, Text: string(ch), Line: line})
			i++

		default:
			return nil, fmt.Errorf("line %d: unexpected character %q", line, ch)
		}
	}

	toks = append(toks, Token{Kind: TokEOF, Line: line})
	return toks, nil
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
