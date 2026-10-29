package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
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
