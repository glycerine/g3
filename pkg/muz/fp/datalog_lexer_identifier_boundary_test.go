package fp

import (
	"strings"
	"testing"
)

func TestDatalogLexerSourceIdentifierPunctuationBoundaries(t *testing.T) {
	lexer, err := newDatalogLexer(strings.NewReader("a!b a<b a>b <word >word < > ! != a!=b"))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		text string
		kind datalogTokenKind
	}{
		{"a!b", datalogIdentifier}, {"a<b", datalogIdentifier}, {"a>b", datalogIdentifier}, {"<word", datalogIdentifier}, {">word", datalogIdentifier}, {"<", datalogLess}, {">", datalogGreater}, {"!", datalogNegation}, {"!=", datalogNotEqual}, {"a!", datalogIdentifier}, {"=", datalogEqual}, {"b", datalogIdentifier},
	}
	for i, expected := range want {
		got := lexer.next()
		if got.text != expected.text || got.kind != expected.kind {
			t.Fatalf("token %d = %#v, want %q kind %v", i, got, expected.text, expected.kind)
		}
	}
	if got := lexer.next(); got.kind != datalogEOF {
		t.Fatal("extra token", got)
	}
}
