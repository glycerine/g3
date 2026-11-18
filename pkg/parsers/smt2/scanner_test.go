package smt2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func scannerTokenDescription(s *Scanner, token Token) string {
	switch token {
	case SymbolToken:
		return "symbol:" + s.ID()
	case KeywordToken:
		return "keyword:" + s.ID()
	case StringToken:
		return "string:" + s.String()
	case IntegerToken:
		return "int:" + scannerNumberText(s)
	case DecimalToken:
		return "decimal:" + scannerNumberText(s)
	case BitVectorToken:
		return fmt.Sprint("bv:", scannerNumberText(s), "/", s.BitVectorSize())
	case LeftParenToken:
		return "("
	case RightParenToken:
		return ")"
	case EOFToken:
		return "eof"
	default:
		return "invalid"
	}
}

func TestSMT2ScannerCommandTokensAndExactNumerals(t *testing.T) {
	s, err := NewScanner(strings.NewReader("; comment\n( :kw |a b| -12 - 12 1.25 #xAf #b0010 \"a\"\"b\" )"), true, testScannerNumerals)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"(", "keyword::kw", "symbol:a b", "symbol:-12", "symbol:-", "int:12", "decimal:5/4", "bv:175/8", "bv:2/4", "string:a\"b", ")", "eof"}
	for i, expected := range want {
		token, err := s.Scan()
		if err != nil {
			t.Fatal(i, err)
		}
		if got := scannerTokenDescription(s, token); got != expected {
			t.Fatal(i, got, "want", expected)
		}
	}
}

func TestSMT2ScannerComplianceQuotedEscapesAndLocations(t *testing.T) {
	for _, compliant := range []bool{false, true} {
		s, err := NewScanner(strings.NewReader("-12.5 "), compliant, testScannerNumerals)
		if err != nil {
			t.Fatal(err)
		}
		token, err := s.Scan()
		if err != nil {
			t.Fatal(err)
		}
		want := "decimal:-25/2"
		if compliant {
			want = "symbol:-12.5"
		}
		if got := scannerTokenDescription(s, token); got != want {
			t.Fatal(compliant, got)
		}
	}
	s, err := NewScanner(strings.NewReader(";ignored\n(foo\n bar)"), true, testScannerNumerals)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		token        Token
		line, column int
	}{{LeftParenToken, 2, 1}, {SymbolToken, 2, 2}, {SymbolToken, 3, 1}, {RightParenToken, 3, 4}} {
		token, err := s.Scan()
		if err != nil {
			t.Fatal(err)
		}
		if token != want.token || s.Line() != want.line || s.Position() != want.column {
			t.Fatal("source token location", token, s.Line(), s.Position(), want)
		}
	}
	s, err = NewScanner(strings.NewReader(`|a\|b| `), true, testScannerNumerals)
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.Scan()
	if err != nil || token != SymbolToken || s.ID() != `a\|b` {
		t.Fatal("quoted symbol escape", token, s.ID(), err)
	}
}

func TestSMT2ScannerInputCachingAndRecovery(t *testing.T) {
	s, err := NewScanner(strings.NewReader(" \t(echo \"a\"\"b\") remainder "), true, testScannerNumerals)
	if err != nil {
		t.Fatal(err)
	}
	s.StartCaching()
	for {
		token, err := s.Scan()
		if err != nil {
			t.Fatal(err)
		}
		if token == RightParenToken {
			break
		}
		if token == EOFToken {
			t.Fatal("missing command end")
		}
	}
	s.StopCaching()
	got, err := s.CachedString(0, s.CacheSize())
	if err != nil || got != `(echo "a""b")` {
		t.Fatal("cached command", got, err)
	}
	if _, err := s.CachedString(-1, 0); err == nil {
		t.Fatal("invalid cache range accepted")
	}
	s.ResetCache()
	if got, err := s.CachedString(0, 0); err != nil || got != "" {
		t.Fatal("empty cache", got, err)
	}
	s, err = NewScanner(strings.NewReader("[ good )"), true, testScannerNumerals)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Scan()
	var scanError *ScannerError
	if !errors.As(err, &scanError) || scanError.Message != "unexpected character" || scanError.Line != 1 || scanError.Column != 1 {
		t.Fatal("scanner exception", err)
	}
	token, err := s.Scan()
	if err != nil || token != SymbolToken || s.ID() != "good" {
		t.Fatal("recovery did not consume the offending byte", token, err)
	}
}

func TestSMT2ScannerMalformedAndEOFGuards(t *testing.T) {
	for _, tc := range []struct{ input, message string }{{"|abc", "unexpected end of quoted symbol"}, {"\"abc", "unexpected end of string"}, {"#q ", "invalid bit-vector literal, expecting 'x' or 'b'"}, {"#x ", "invalid empty bit-vector literal"}, {"#b ", "invalid empty bit-vector literal"}} {
		s, err := NewScanner(strings.NewReader(tc.input), true, testScannerNumerals)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Scan()
		var failure *ScannerError
		if !errors.As(err, &failure) || failure.Message != tc.message {
			t.Fatal(tc.input, err)
		}
	}
	for _, tc := range []struct{ input, want string }{{"word", "eof"}, {"12", "int:12"}, {"#b10", "bv:2/2"}, {"\"ok\"", "string:ok"}} {
		s, err := NewScanner(strings.NewReader(tc.input), true, testScannerNumerals)
		if err != nil {
			t.Fatal(err)
		}
		token, err := s.Scan()
		if err != nil {
			t.Fatal(err)
		}
		if got := scannerTokenDescription(s, token); got != tc.want {
			t.Fatal(tc.input, got, tc.want)
		}
		if token, err = s.Scan(); err != nil || token != EOFToken {
			t.Fatal("repeated EOF", token, err)
		}
	}
	if _, err := NewScanner(failingScannerReader{}, true, testScannerNumerals); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal("input error was lost", err)
	}
}

type failingScannerReader struct{}

func (failingScannerReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestSMT2ScannerSourceTraceAndLargeTokens(t *testing.T) {
	s, err := NewScanner(strings.NewReader("|quoted| plain -1.25 "), false, testScannerNumerals)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("scanner")
	s.traceTags = tags
	for i := 0; i < 3; i++ {
		if _, err := s.Scan(); err != nil {
			t.Fatal(err)
		}
	}
	if tags.ShouldTrace("scanner") {
		want := "-------- [scanner] read_quoted_symbol src/parsers/smt2/smt2scanner.cpp:85 ---------\nnew quoted symbol: quoted\n------------------------------------------------\n" +
			"-------- [scanner] read_symbol_core src/parsers/smt2/smt2scanner.cpp:105 ---------\nnew symbol: plain\n------------------------------------------------\n" +
			"-------- [scanner] read_number src/parsers/smt2/smt2scanner.cpp:147 ---------\nnew number: 5/4\n------------------------------------------------\n"
		if output.String() != want {
			t.Fatal("source scanner trace", output.String())
		}
	} else if output.Len() != 0 {
		t.Fatal("disabled trace wrote output")
	}
	large := strings.Repeat("x", 5000)
	s, err = NewScanner(strings.NewReader("|"+large+"| "), true, testScannerNumerals)
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.Scan()
	if err != nil || token != SymbolToken || s.ID() != large {
		t.Fatal("token crossing input buffers", token, len(s.ID()), err)
	}
}
