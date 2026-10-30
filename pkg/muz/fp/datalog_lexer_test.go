package fp

import (
	"strings"
	"testing"
)

func TestDatalogLexerSourceTokensAndLocations(t *testing.T) {
	lexer, err := newDatalogLexer(strings.NewReader("P(X) :- !Q(\"a\"), X != 3. # tail\n"))
	if err != nil {
		t.Fatal(err)
	}
	var kinds []datalogTokenKind
	for {
		token := lexer.next()
		kinds = append(kinds, token.kind)
		if token.kind == datalogEOF {
			break
		}
	}
	want := []datalogTokenKind{datalogIdentifier, datalogLeftParen, datalogIdentifier, datalogRightParen, datalogLeftArrow, datalogNegation, datalogIdentifier, datalogLeftParen, datalogString, datalogRightParen, datalogComma, datalogIdentifier, datalogNotEqual, datalogNumber, datalogPeriod, datalogNewline, datalogEOF}
	if len(kinds) != len(want) {
		t.Fatalf("token count %d: %v", len(kinds), kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("token %d = %v, want %v", i, kinds[i], want[i])
		}
	}
}

func TestDatalogLexerQuotedIdentifiersAndFailures(t *testing.T) {
	lexer, _ := newDatalogLexer(strings.NewReader("|name with spaces| \"unterminated\n"))
	if token := lexer.next(); token.kind != datalogIdentifier || token.text != "name with spaces" {
		t.Fatal(token)
	}
	if token := lexer.next(); token.kind != datalogError || token.line != 1 {
		t.Fatal(token)
	}
}
