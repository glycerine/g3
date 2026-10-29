package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestSMTContextBitVectorCoreAPI(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.BitVectorSort(8))
	if size, err := f.c.BitVectorSortSize(sort); err != nil || size != 8 {
		t.Fatal("size", size, err)
	}
	a := f.pin(f.c.BitVectorValue(7, sort))
	b := f.pin(f.c.BitVectorValue(-1, sort))
	checks := []struct {
		kind int32
		term ast.ASTHandle
	}{
		{32, f.pin(f.c.BitVectorNot(a))}, {30, f.pin(f.c.BitVectorAnd(a, b))}, {31, f.pin(f.c.BitVectorOr(a, b))}, {33, f.pin(f.c.BitVectorXor(a, b))},
		{34, f.pin(f.c.BitVectorNand(a, b))}, {35, f.pin(f.c.BitVectorNor(a, b))}, {36, f.pin(f.c.BitVectorXnor(a, b))},
		{4, f.pin(f.c.BitVectorAdd(a, b))}, {5, f.pin(f.c.BitVectorSub(a, b))}, {6, f.pin(f.c.BitVectorMul(a, b))},
		{8, f.pin(f.c.BitVectorUDiv(a, b))}, {9, f.pin(f.c.BitVectorSRem(a, b))}, {11, f.pin(f.c.BitVectorSMod(a, b))},
		{22, f.pin(f.c.BitVectorULE(a, b))}, {27, f.pin(f.c.BitVectorSLT(a, b))}, {45, f.pin(f.c.BitVectorSHL(a, b))},
	}
	for _, check := range checks {
		if kind, err := f.m.GetDeclKind(check.term); err != nil || kind != check.kind {
			t.Fatal("kind", check.kind, kind, err)
		}
	}
	concat := f.pin(f.c.BitVectorConcat(a, b))
	if size, err := f.m.BitVectorSize(concat); err != nil || size != 16 {
		t.Fatal("concat size", size, err)
	}
	extract := f.pin(f.c.BitVectorExtract(5, 2, a))
	if size, err := f.m.BitVectorSize(extract); err != nil || size != 4 {
		t.Fatal("extract size", size, err)
	}
	for _, term := range []ast.ASTHandle{f.pin(f.c.BitVectorSignExtend(2, a)), f.pin(f.c.BitVectorZeroExtend(2, a)), f.pin(f.c.BitVectorRepeat(2, a)), f.pin(f.c.BitVectorRotateLeft(3, a)), f.pin(f.c.BitVectorRotateRight(3, a))} {
		if yes, err := f.m.IsBitVector(term); err != nil || !yes {
			t.Fatal("parameterized", yes, err)
		}
	}
}

func TestSMTContextBitVectorIntegerConversions(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	integer := f.pin(f.m.ArithmeticIntegerNumeralText("19"))
	bits := f.pin(f.c.IntToBitVector(8, integer))
	back := f.pin(f.c.BitVectorToInt(bits))
	sort := f.pin(f.m.GetSort(bits))
	if size, err := f.c.BitVectorSortSize(sort); err != nil || size != 8 {
		t.Fatal("conversion size", size, err)
	}
	backSort := f.pin(f.m.GetSort(back))
	if yes, err := f.m.IsBitVectorSort(backSort); err != nil || yes {
		t.Fatal("bv2int sort", yes, err)
	}
	signed := f.pin(f.c.BitVectorToInt(bits, true))
	if kind, err := f.m.GetDeclKind(signed); err != nil || kind != ast.ASTOpIte {
		t.Fatal("signed bv2int", kind, err)
	}
	if _, err := f.c.BitVectorSort(0); err != ast.ErrASTDomain {
		t.Fatal("zero width", err)
	}
}
