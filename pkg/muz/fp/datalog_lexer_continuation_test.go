package fp

import (
	"strings"
	"testing"
)

func TestDatalogLexerSourceBackslashContinuation(t *testing.T) {
	lexer, err := newDatalogLexer(strings.NewReader("P \\   \n Q \\ # continued\n R \\name"))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		kind datalogTokenKind
		text string
	}{
		{datalogIdentifier, "P"}, {datalogIdentifier, "Q"},
		{datalogIdentifier, "R"}, {datalogIdentifier, "\\name"}, {datalogEOF, ""},
	}
	for i, expected := range want {
		got := lexer.next()
		if got.kind != expected.kind || got.text != expected.text {
			t.Fatalf("token %d = %v %q, want %v %q", i, got.kind, got.text, expected.kind, expected.text)
		}
	}
}
