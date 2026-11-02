package fp

// Source: src/muz/fp/datalog_parser.cpp dtoken, reserved_symbols and dlexer.

import (
	"io"
	"strings"
	"unicode"
)

type datalogTokenKind uint8

const (
	datalogError datalogTokenKind = iota
	datalogEOF
	datalogNewline
	datalogLeftParen
	datalogRightParen
	datalogString
	datalogIdentifier
	datalogNumber
	datalogPeriod
	datalogInclude
	datalogComma
	datalogColon
	datalogWildcard
	datalogLeftArrow
	datalogNotEqual
	datalogLess
	datalogGreater
	datalogEqual
	datalogNegation
)

type datalogToken struct {
	kind         datalogTokenKind
	text         string
	line, column int
}

type datalogLexer struct {
	input          []rune
	position       int
	line, column   int
	parsingDomains bool
}

func newDatalogLexer(input io.Reader) (*datalogLexer, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	return &datalogLexer{input: []rune(string(data)), line: 1, column: 1}, nil
}

func newStringReader(input string) io.Reader { return strings.NewReader(input) }

func (l *datalogLexer) peek() rune {
	if l.position == len(l.input) {
		return -1
	}
	return l.input[l.position]
}

func (l *datalogLexer) take() rune {
	r := l.peek()
	if r < 0 {
		return r
	}
	l.position++
	if r == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return r
}

func (l *datalogLexer) next() datalogToken {
	for l.peek() == ' ' || l.peek() == '\t' || l.peek() == '\r' {
		l.take()
	}
	line, column := l.line, l.column
	r := l.take()
	switch r {
	case -1:
		return datalogToken{kind: datalogEOF, line: line, column: column}
	case '\n':
		return datalogToken{kind: datalogNewline, text: "\\n", line: line, column: column}
	case '#':
		lineComment := column == 1
		for l.peek() >= 0 && l.peek() != '\n' {
			l.take()
		}
		// Source read_comment consumes the newline only when '#' begins the
		// physical line. This lets comment-only lines occur among domains,
		// while a trailing comment still exposes the line terminator.
		if lineComment && l.peek() == '\n' {
			l.take()
		}
		return l.next()
	case '(':
		return datalogToken{kind: datalogLeftParen, text: "(", line: line, column: column}
	case ')':
		return datalogToken{kind: datalogRightParen, text: ")", line: line, column: column}
	case ',':
		return datalogToken{kind: datalogComma, text: ",", line: line, column: column}
	case ':':
		if l.peek() == '-' {
			l.take()
			return datalogToken{kind: datalogLeftArrow, text: ":-", line: line, column: column}
		}
		return datalogToken{kind: datalogColon, text: ":", line: line, column: column}
	case '!':
		if l.peek() == '=' {
			l.take()
			return datalogToken{kind: datalogNotEqual, text: "!=", line: line, column: column}
		}
		return datalogToken{kind: datalogNegation, text: "!", line: line, column: column}
	case '=':
		return datalogToken{kind: datalogEqual, text: "=", line: line, column: column}
	case '<':
		return datalogToken{kind: datalogLess, text: "<", line: line, column: column}
	case '>':
		return datalogToken{kind: datalogGreater, text: ">", line: line, column: column}
	case '"':
		var value strings.Builder
		for l.peek() >= 0 && l.peek() != '"' && l.peek() != '\n' {
			value.WriteRune(l.take())
		}
		if l.peek() != '"' {
			return datalogToken{kind: datalogError, text: "unterminated string", line: line, column: column}
		}
		l.take()
		return datalogToken{kind: datalogString, text: value.String(), line: line, column: column}
	case '|':
		var value strings.Builder
		for l.peek() >= 0 && l.peek() != '|' {
			value.WriteRune(l.take())
		}
		if l.peek() != '|' {
			return datalogToken{kind: datalogError, text: "unterminated quoted identifier", line: line, column: column}
		}
		l.take()
		return datalogToken{kind: datalogIdentifier, text: value.String(), line: line, column: column}
	}
	if unicode.IsDigit(r) || r == '-' && unicode.IsDigit(l.peek()) {
		var value strings.Builder
		value.WriteRune(r)
		for unicode.IsDigit(l.peek()) {
			value.WriteRune(l.take())
		}
		return datalogToken{kind: datalogNumber, text: value.String(), line: line, column: column}
	}
	var value strings.Builder
	value.WriteRune(r)
	for {
		p := l.peek()
		if p < 0 || unicode.IsSpace(p) || strings.ContainsRune("(),:#=<>!", p) || (!l.parsingDomains && p == '.') {
			break
		}
		value.WriteRune(l.take())
	}
	text := value.String()
	switch text {
	case ".":
		return datalogToken{kind: datalogPeriod, text: text, line: line, column: column}
	case ".include":
		return datalogToken{kind: datalogInclude, text: text, line: line, column: column}
	case "_":
		return datalogToken{kind: datalogWildcard, text: text, line: line, column: column}
	}
	return datalogToken{kind: datalogIdentifier, text: text, line: line, column: column}
}
