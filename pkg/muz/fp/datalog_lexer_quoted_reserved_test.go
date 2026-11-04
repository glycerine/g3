package fp

import (
	"strings"
	"testing"
)

func TestDatalogLexerQuotedReservedSymbols(t *testing.T) {
	cases := []struct {
		text string
		kind datalogTokenKind
	}{
		{":-", datalogLeftArrow}, {"_", datalogWildcard}, {".", datalogPeriod}, {"!=", datalogNotEqual}, {"=", datalogEqual}, {"<", datalogLess}, {">", datalogGreater}, {":", datalogColon}, {".include", datalogInclude}, {"!", datalogNegation}, {"ordinary name", datalogIdentifier},
	}
	for _, tc := range cases {
		lexer, err := newDatalogLexer(strings.NewReader("|" + tc.text + "|"))
		if err != nil {
			t.Fatal(err)
		}
		got := lexer.next()
		if got.kind != tc.kind || got.text != tc.text {
			t.Fatalf("quoted %q: %#v", tc.text, got)
		}
	}
	lexer, err := newDatalogLexer(strings.NewReader("|unclosed"))
	if err != nil {
		t.Fatal(err)
	}
	got := lexer.next()
	if got.kind != datalogIdentifier || got.text != "unclosed" {
		t.Fatal("quoted EOF", got)
	}
}

func TestDatalogLexerNULTerminatesSourceTokenText(t *testing.T) {
	for _, tc := range []struct {
		input, text string
		kind        datalogTokenKind
	}{
		{"|_\x00ignored|,", "_", datalogWildcard},
		{"|.\x00ignored|,", ".", datalogPeriod},
		{"|name\x00ignored|,", "name", datalogIdentifier},
		{"name\x00ignored,", "name", datalogIdentifier},
		{"\"name\x00ignored\",", "name", datalogString},
	} {
		lexer, err := newDatalogLexer(strings.NewReader(tc.input))
		if err != nil {
			t.Fatal(err)
		}
		got := lexer.next()
		if got.kind != tc.kind || got.text != tc.text {
			t.Fatalf("%q: got %#v", tc.input, got)
		}
		if next := lexer.next(); next.kind != datalogComma {
			t.Fatalf("%q: suffix was not consumed: %#v", tc.input, next)
		}
	}
}
