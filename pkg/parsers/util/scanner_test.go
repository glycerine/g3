package util

import (
	"bytes"
	"strings"
	"testing"
)

func TestScannerSMT2TokensNumbersSymbolsStringsAndComments(t *testing.T) {
	input := "(foo :bar 12 3.25 #x0f #b101 |quoted symbol| \"a\\\"b\" {note}\n; skip\n)"
	var diagnostics bytes.Buffer
	s := NewScanner(strings.NewReader(input), &diagnostics, true, true, testScannerNumerals)
	want := []Token{LeftParen, Identifier, Colon, Identifier, Integer, Float, BitVector, BitVector, Identifier, String, Comment, RightParen, EndOfFile}
	for i, token := range want {
		if got := s.Scan(); got != token {
			t.Fatalf("token %d: got %v want %v", i, got, token)
		}
		switch i {
		case 1:
			if s.ID() != "foo" {
				t.Fatal(s.ID())
			}
		case 4:
			if scannerNumberText(s) != "12" {
				t.Fatal(scannerNumberText(s))
			}
		case 5:
			if scannerNumberText(s) != "13/4" {
				t.Fatal(scannerNumberText(s))
			}
		case 6:
			if scannerNumberText(s) != "15" || s.BitVectorSize() != 8 {
				t.Fatal(scannerNumberText(s), s.BitVectorSize())
			}
		case 7:
			if scannerNumberText(s) != "5" || s.BitVectorSize() != 3 {
				t.Fatal(scannerNumberText(s), s.BitVectorSize())
			}
		case 8:
			if s.ID() != "quoted symbol" {
				t.Fatal(s.ID())
			}
		case 9:
			if s.ID() != "a\\\"b" {
				t.Fatal(s.ID())
			}
		case 10:
			if s.ID() != "note" {
				t.Fatal(s.ID())
			}
		}
	}
	if diagnostics.Len() != 0 || s.Line() != 3 {
		t.Fatal(diagnostics.String(), s.Line())
	}
}

func TestScannerLegacyNegativeParametersAndBitVectorCompatibility(t *testing.T) {
	s := NewScanner(strings.NewReader("-20 op[12:name:3] #xF"), nil, false, false, testScannerNumerals)
	if s.Scan() != Integer || scannerNumberText(s) != "-20" {
		t.Fatal("negative", scannerNumberText(s))
	}
	if s.Scan() != Identifier || s.ID() != "op" {
		t.Fatal("identifier", s.ID())
	}
	params := s.Parameters()
	if len(params) != 3 || params[0].Number == nil || *params[0].Number != 12 || params[1].Symbol != "name" || params[2].Number == nil || *params[2].Number != 3 {
		t.Fatal("parameters", params)
	}
	if s.Scan() != Identifier || s.ID() != "#" {
		t.Fatal("legacy hash", s.ID())
	}
	if s.Scan() != Identifier || s.ID() != "xF" {
		t.Fatal("legacy suffix", s.ID())
	}
	if s.Scan() != EndOfFile {
		t.Fatal("eof")
	}

	compat := NewScanner(strings.NewReader("#xF #b10"), nil, true, false, testScannerNumerals)
	if compat.Scan() != Identifier || compat.ID() != "bvhexF" {
		t.Fatal(compat.ID())
	}
	if compat.Scan() != Identifier || compat.ID() != "bvbin10" {
		t.Fatal(compat.ID())
	}
}

func TestScannerErrorsAreTerminal(t *testing.T) {
	var diagnostics bytes.Buffer
	s := NewScanner(strings.NewReader("#q"), &diagnostics, true, true, testScannerNumerals)
	if s.Scan() != ScanError || s.Scan() != ScanError || diagnostics.Len() != 0 {
		t.Fatal("bit-vector error", diagnostics.String())
	}
	s = NewScanner(strings.NewReader("|unterminated"), &diagnostics, true, false, testScannerNumerals)
	if s.Scan() != EndOfFile || !strings.Contains(diagnostics.String(), "unexpected end of file") {
		t.Fatal("quoted error", diagnostics.String())
	}
}
