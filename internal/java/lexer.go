// Package java reads the declarations of Java source files: packages, imports, types, fields,
// methods, annotations and generics. Method bodies and field initializers are skipped (except
// simple constant initializers), which is all httpgen needs and keeps the parser small.
package java

import (
	"fmt"
	"strings"
)

// TokenKind classifies a token.
type TokenKind int

const (
	EOF TokenKind = iota
	Ident
	String // value holds the decoded text
	Char
	Number
	Punct
)

// Token is one lexical unit with its position.
type Token struct {
	Kind  TokenKind
	Text  string // source text, or the decoded value for strings
	Line  int
	Col   int
	Space bool // preceded by whitespace or a comment
}

// ParseError points at the file position that could not be read.
type ParseError struct {
	Path string
	Line int
	Col  int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.Path, e.Line, e.Col, e.Msg)
}

// multi-character operators, longest first.
var operators = []string{">>>=", "<<=", ">>=", "...", "->", "::", "==", "!=", "<=", ">=", "&&", "||", "++", "--",
	"+=", "-=", "*=", "/=", "&=", "|=", "^=", "%=", "<<"}

// Lex splits source into tokens. `>` is always a single token so nested generics close cleanly.
func Lex(path, src string) ([]Token, error) {
	var toks []Token
	line, col := 1, 1
	i := 0
	space := true
	advance := func(n int) {
		for k := 0; k < n && i < len(src); k++ {
			if src[i] == '\n' {
				line++
				col = 1
			} else {
				col++
			}
			i++
		}
	}
	errAt := func(msg string) error { return &ParseError{Path: path, Line: line, Col: col, Msg: msg} }

	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f':
			advance(1)
			space = true
			continue
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				advance(1)
			}
			space = true
			continue
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, errAt("unterminated comment")
			}
			advance(end + 4)
			space = true
			continue
		}

		startLine, startCol := line, col
		emit := func(kind TokenKind, text string) {
			toks = append(toks, Token{Kind: kind, Text: text, Line: startLine, Col: startCol, Space: space})
			space = false
		}
		switch {
		case isIdentStart(c):
			j := i
			for j < len(src) && isIdentPart(src[j]) {
				j++
			}
			emit(Ident, src[i:j])
			advance(j - i)
		case c >= '0' && c <= '9' || (c == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9'):
			j := i
			for j < len(src) && (isIdentPart(src[j]) || src[j] == '.' ||
				((src[j] == '+' || src[j] == '-') && (src[j-1] == 'e' || src[j-1] == 'E' || src[j-1] == 'p' || src[j-1] == 'P'))) {
				j++
			}
			emit(Number, src[i:j])
			advance(j - i)
		case strings.HasPrefix(src[i:], `"""`):
			end := strings.Index(src[i+3:], `"""`)
			if end < 0 {
				return nil, errAt("unterminated text block")
			}
			emit(String, textBlock(src[i+3:i+3+end]))
			advance(end + 6)
		case c == '"' || c == '\'':
			j := i + 1
			var b strings.Builder
			for j < len(src) && src[j] != c {
				if src[j] == '\n' {
					return nil, errAt("unterminated literal")
				}
				if src[j] == '\\' && j+1 < len(src) {
					b.WriteString(unescape(src[j+1]))
					j += 2
					continue
				}
				b.WriteByte(src[j])
				j++
			}
			if j >= len(src) {
				return nil, errAt("unterminated literal")
			}
			if c == '"' {
				emit(String, b.String())
			} else {
				emit(Char, b.String())
			}
			advance(j + 1 - i)
		default:
			op := string(c)
			for _, o := range operators {
				if strings.HasPrefix(src[i:], o) {
					op = o
					break
				}
			}
			emit(Punct, op)
			advance(len(op))
		}
	}
	toks = append(toks, Token{Kind: EOF, Line: line, Col: col, Space: true})
	return toks, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentPart(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }

func unescape(c byte) string {
	switch c {
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case 'r':
		return "\r"
	case 'b':
		return "\b"
	case 'f':
		return "\f"
	case 's':
		return " "
	case '0':
		return "\x00"
	default:
		return string(c)
	}
}

// textBlock strips the incidental indentation of a Java text block.
func textBlock(raw string) string {
	lines := strings.Split(strings.TrimPrefix(raw, "\n"), "\n")
	indent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	for k, l := range lines {
		if len(l) >= indent && indent > 0 {
			lines[k] = l[indent:]
		}
	}
	return strings.Join(lines, "\n")
}
