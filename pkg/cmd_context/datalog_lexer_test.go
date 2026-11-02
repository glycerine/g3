package cmd_context

import "testing"

func TestDatalogLexerSourceTokensCommentsContinuationAndQuotes(t *testing.T) {
	lexer := newDatalogLexer([]byte("P(|x y|,\"z\") :- !Q(_), \\\n X != 2. # tail\n"))
	want := []datalogToken{dlIdentifier, dlLeftParen, dlIdentifier, dlComma, dlString, dlRightParen, dlLeftArrow, dlNegation, dlIdentifier, dlLeftParen, dlWildcard, dlRightParen, dlComma, dlIdentifier, dlNotEqual, dlNumber, dlPeriod, dlNewline, dlEOF}
	for i, expected := range want {
		got, err := lexer.next()
		if err != nil {
			t.Fatal(i, err)
		}
		if got.kind != expected {
			t.Fatalf("token %d = %v %q, want %v", i, got.kind, got.text, expected)
		}
	}
}

func TestDatalogLexerRejectsUnterminatedQuotedValues(t *testing.T) {
	for _, input := range []string{"\"value", "|name"} {
		lexer := newDatalogLexer([]byte(input))
		if _, err := lexer.next(); err == nil {
			t.Fatal("accepted unterminated token", input)
		}
	}
}

func TestDatalogLexerSourceCommentNewlinePolicy(t *testing.T) {
	lexer := newDatalogLexer([]byte("# first\nN 2 # trailing\n# between\nM 3\n"))
	lexer.parsingDomains = true
	want := []struct {
		kind datalogToken
		text string
	}{
		{dlIdentifier, "N"}, {dlNumber, "2"}, {dlNewline, ""},
		{dlIdentifier, "M"}, {dlNumber, "3"}, {dlNewline, ""}, {dlEOF, ""},
	}
	for i, expected := range want {
		got, err := lexer.next()
		if err != nil {
			t.Fatal(i, err)
		}
		if got.kind != expected.kind || got.text != expected.text {
			t.Fatalf("token %d = %v %q, want %v %q", i, got.kind, got.text, expected.kind, expected.text)
		}
	}
}
