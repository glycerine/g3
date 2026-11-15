package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextNumeralConstructionAndQueries(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	integerSeed := f.pin(f.m.ArithmeticIntegerNumeralText("0"))
	integerSort := f.pin(f.m.GetSort(integerSeed))
	realSeed := f.pin(f.m.ArithmeticRealNumeralText("0"))
	realSort := f.pin(f.m.GetSort(realSeed))
	bvSort := f.pin(f.c.BitVectorSort(8))
	integer := f.pin(f.c.IntNumeral(-42, integerSort))
	real := f.pin(f.c.Numeral("7/3", realSort))
	bv := f.pin(f.c.UintNumeral(257, bvSort))
	for _, tc := range []struct {
		term ast.ASTHandle
		want string
	}{{integer, "-42"}, {real, "7/3"}, {bv, "1"}} {
		text, err := f.c.NumeralString(tc.term)
		if err != nil || text != tc.want {
			t.Fatal("numeral", text, tc.want, err)
		}
		if yes, err := f.c.IsNumeral(tc.term); err != nil || !yes {
			t.Fatal("recognition", yes, err)
		}
	}
	if value, yes, err := f.c.NumeralInt64(integer); err != nil || !yes || value != -42 {
		t.Fatal("int64", value, yes, err)
	}
	if num, den, yes, err := f.c.NumeralRationalInt64(real); err != nil || !yes || num != 7 || den != 3 {
		t.Fatal("rational", num, den, yes, err)
	}
	if value, yes, err := f.c.NumeralUint64(bv); err != nil || !yes || value != 1 {
		t.Fatal("uint64", value, yes, err)
	}
}

func TestSMTContextBitVectorNumeralBits(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	value := f.pin(f.c.BitVectorNumeral([]bool{true, false, true, false}))
	text, err := f.c.NumeralString(value)
	if err != nil || text != "5" {
		t.Fatal(text, err)
	}
	if size, err := f.m.BitVectorSize(value); err != nil || size != 4 {
		t.Fatal("size", size, err)
	}
	if _, err := f.c.BitVectorNumeral(nil); err != ast.ErrASTDomain {
		t.Fatal("empty bits", err)
	}
}

// AUDIT api_02-F1: Z3_mk_numeral filters the text for every sort before
// building anything; p, P and + are allowed only for floating-point sorts.
// Expected outcomes are what build/libz3.so returns (NULL with
// Z3_PARSER_ERROR, or the printed numeral).
func TestAPINumeralRejectsNonNumeralText(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	intSort := f.pin(f.c.IntegerSort())
	realSort := f.pin(f.c.RealSort())
	bv8 := f.pin(f.c.BitVectorSort(8))
	fp := f.pin(f.c.FloatingPointSort(8, 24))
	rm := f.pin(f.c.FloatingPointRoundingModeSort())
	sorts := map[string]ast.ASTHandle{"Int": intSort, "Real": realSort, "BV8": bv8, "FP": fp, "RM": rm}
	for _, name := range []string{"Int", "Real", "BV8", "FP", "RM"} {
		for _, text := range []string{"abc", "0x10", "1_000"} {
			if _, err := f.c.Numeral(text, sorts[name]); !errors.Is(err, errAPINumeralParse) {
				t.Fatalf("%s %q: want parser error, got %v", name, text, err)
			}
		}
	}
	for _, name := range []string{"Int", "Real", "BV8", "RM"} {
		for _, text := range []string{"+5", "1.5p+1"} {
			if _, err := f.c.Numeral(text, sorts[name]); !errors.Is(err, errAPINumeralParse) {
				t.Fatalf("%s %q: want parser error, got %v", name, text, err)
			}
		}
	}
	for _, tc := range []struct {
		name, text, want string
	}{
		{"Int", "-7", "(- 7)"},
		{"Int", "1e2", "100"},
		{"Real", "1/2", "(/ 1.0 2.0)"},
		{"BV8", "-7", "#xf9"},
		{"FP", "+5", "(fp #b0 #x81 #b01000000000000000000000)"},
		{"FP", "1.5p+1", "(fp #b0 #x80 #b10000000000000000000000)"},
	} {
		h, err := f.c.Numeral(tc.text, sorts[tc.name])
		if err != nil {
			t.Fatalf("%s %q: %v", tc.name, tc.text, err)
		}
		f.pin(h, nil)
		got, err := f.c.ASTString(h, APIASTPrintSMT2)
		astTestMust(t, err)
		if got != tc.want {
			t.Fatalf("%s %q: got %q want %q", tc.name, tc.text, got, tc.want)
		}
	}
	// check_numeral_sort runs before the filter (Z3_INVALID_ARG).
	if _, err := f.c.Numeral("abc", f.pin(f.c.BoolSort())); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-numeral sort", err)
	}
}
