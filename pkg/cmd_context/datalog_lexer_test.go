package cmd_context

import "testing"

type dlexeme struct {
	tok  dtoken
	data string
}

func datalogLexString(input string, domains bool) []dlexeme {
	lexer := newDlexer()
	lexer.parsingDomains = domains
	lexer.setStream(&datalogStreamInput{data: []byte(input)}, nil)
	var result []dlexeme
	for {
		tok := lexer.nextToken()
		result = append(result, dlexeme{tok, lexer.tokenData()})
		if tok == dtkEOS || tok == dtkError {
			return result
		}
	}
}

func checkDatalogLex(t *testing.T, input string, domains bool, want []dlexeme) {
	t.Helper()
	got := datalogLexString(input, domains)
	if len(got) != len(want) {
		t.Fatalf("%q: tokens %v, want %v", input, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%q: token %d = %v %q, want %v %q", input, i, got[i].tok, got[i].data, want[i].tok, want[i].data)
		}
	}
}

func TestDatalogLexerSourceTokensCommentsContinuationAndQuotes(t *testing.T) {
	checkDatalogLex(t, "P(|x y|,\"z\") :- !Q(_), \\\n X != 2. # tail\n", false, []dlexeme{
		{dtkID, "P"}, {dtkLP, ""}, {dtkID, "x y"}, {dtkComma, ""}, {dtkString, "z"}, {dtkRP, ""},
		{dtkLeftArrow, ""}, {dtkNeg, ""}, {dtkID, "Q"}, {dtkLP, ""}, {dtkWild, "_"}, {dtkRP, ""}, {dtkComma, ""},
		{dtkID, "X"}, {dtkNEQ, ""}, {dtkNum, "2"}, {dtkPeriod, "."}, {dtkNewline, ""}, {dtkEOS, ""},
	})
}

// read_string fails on an unterminated string; read_bid simply stops at
// the end of the input.
func TestDatalogLexerUnterminatedQuotedValues(t *testing.T) {
	checkDatalogLex(t, "\"value", false, []dlexeme{{dtkError, "value"}})
	checkDatalogLex(t, "\"value\nP", false, []dlexeme{{dtkError, "value"}})
	checkDatalogLex(t, "|name", false, []dlexeme{{dtkID, "name"}, {dtkEOS, "name"}})
}

func TestDatalogLexerSourceCommentNewlinePolicy(t *testing.T) {
	checkDatalogLex(t, "# first\nN 2 # trailing\n# between\nM 3\n", true, []dlexeme{
		{dtkID, "N"}, {dtkNum, "2"}, {dtkNewline, ""},
		{dtkID, "M"}, {dtkNum, "3"}, {dtkNewline, ""}, {dtkEOS, ""},
	})
}

// AUDIT.md cmd_context_01-F9: read_id stops only at ( ) # , : = '.' and
// white space, so < > ! are identifier characters; <, > and _ are reserved
// only as whole tokens, also inside |..|.
func TestDatalogLexerIdentifierTerminators(t *testing.T) {
	checkDatalogLex(t, "X<Y", false, []dlexeme{{dtkID, "X<Y"}, {dtkEOS, "X<Y"}})
	checkDatalogLex(t, "X < Y", false, []dlexeme{{dtkID, "X"}, {dtkLT, "<"}, {dtkID, "Y"}, {dtkEOS, "Y"}})
	checkDatalogLex(t, "X > Y", false, []dlexeme{{dtkID, "X"}, {dtkGT, ">"}, {dtkID, "Y"}, {dtkEOS, "Y"}})
	checkDatalogLex(t, "X!=Y", false, []dlexeme{{dtkID, "X!"}, {dtkEQ, ""}, {dtkID, "Y"}, {dtkEOS, "Y"}})
	checkDatalogLex(t, "|_| |<|", false, []dlexeme{{dtkWild, "_"}, {dtkLT, "<"}, {dtkEOS, "<"}})
	checkDatalogLex(t, "a.b", false, []dlexeme{{dtkID, "a"}, {dtkID, ".b"}, {dtkEOS, ".b"}})
	checkDatalogLex(t, "a. b", false, []dlexeme{{dtkID, "a"}, {dtkPeriod, "."}, {dtkID, "b"}, {dtkEOS, "b"}})
	checkDatalogLex(t, "a.b", true, []dlexeme{{dtkID, "a.b"}, {dtkEOS, "a.b"}})
	checkDatalogLex(t, "-12x", false, []dlexeme{{dtkNum, "-12"}, {dtkID, "x"}, {dtkEOS, "x"}})
}

// The backslash continuation keeps the spaces it looked past in the token,
// and a comment after it continues the line.
func TestDatalogLexerBackslashContinuation(t *testing.T) {
	checkDatalogLex(t, "\\  x", false, []dlexeme{{dtkID, "\\  x"}, {dtkEOS, "\\  x"}})
	checkDatalogLex(t, "a \\ #comment\nb", false, []dlexeme{{dtkID, "a"}, {dtkID, "b"}, {dtkEOS, "b"}})
	checkDatalogLex(t, "a \\  \nb", false, []dlexeme{{dtkID, "a"}, {dtkID, "b"}, {dtkEOS, "b"}})
}

// char_reader ends every line, also the last one, with '\n', and a 0xFF
// byte is the C++ char EOF.
func TestDatalogCharReaderLines(t *testing.T) {
	r := &datalogCharReader{lineReader: &datalogLineReader{data: []byte("ab\nc"), ok: true}}
	var got []int
	for c := r.get(); c != dlEOF; c = r.get() {
		got = append(got, c)
	}
	if want := []int{'a', 'b', '\n', 'c', '\n'}; len(got) != len(want) || got[0] != want[0] || got[2] != want[2] || got[3] != want[3] || got[4] != want[4] {
		t.Fatal("char_reader", got)
	}
	r = &datalogCharReader{lineReader: &datalogLineReader{data: []byte("a\n"), ok: true}}
	got = got[:0]
	for c := r.get(); c != dlEOF; c = r.get() {
		got = append(got, c)
	}
	if len(got) != 3 || got[1] != '\n' || got[2] != '\n' {
		t.Fatal("char_reader trailing newline", got)
	}
	checkDatalogLex(t, "a\xffb", false, []dlexeme{{dtkID, "a"}, {dtkEOS, "a"}})
}
