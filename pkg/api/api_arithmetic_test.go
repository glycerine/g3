package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestSMTContextArithmeticSortsAndFactories(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	ints := f.pin(f.c.IntegerSort())
	reals := f.pin(f.c.RealSort())
	if kind, err := f.m.GetDeclKind(ints); err != nil || kind != 1 {
		t.Fatal("int sort", kind, err)
	}
	if kind, err := f.m.GetDeclKind(reals); err != nil || kind != 0 {
		t.Fatal("real sort", kind, err)
	}
	a := f.pin(f.c.IntNumeral(7, ints))
	b := f.pin(f.c.IntNumeral(3, ints))
	r := f.pin(f.c.Real(7, 3))
	boolean := f.pin(f.m.MkBoolSort())
	operations := []struct {
		kind int32
		term ast.ASTHandle
	}{{6, f.pin(f.c.ArithmeticAdd(a, b))}, {9, f.pin(f.c.ArithmeticMul(a, b))}, {7, f.pin(f.c.ArithmeticSub(a, b))}, {8, f.pin(f.c.ArithmeticNeg(a))}, {11, f.pin(f.c.ArithmeticDiv(a, b))}, {13, f.pin(f.c.ArithmeticMod(a, b))}, {12, f.pin(f.c.ArithmeticRem(a, b))}, {18, f.pin(f.c.ArithmeticPower(a, b))}}
	for _, op := range operations {
		if kind, err := f.m.GetDeclKind(op.term); err != nil || kind != op.kind {
			t.Fatal("operation", op.kind, kind, err)
		}
	}
	for _, term := range []ast.ASTHandle{f.pin(f.c.ArithmeticLT(a, b)), f.pin(f.c.ArithmeticGT(a, b)), f.pin(f.c.ArithmeticLE(a, b)), f.pin(f.c.ArithmeticGE(a, b)), f.pin(f.c.IsIntegerValue(r))} {
		if sort := f.pin(f.m.GetSort(term)); sort != boolean {
			t.Fatal("predicate sort", sort, boolean)
		}
	}
	realA := f.pin(f.c.IntegerToReal(a))
	if sort := f.pin(f.m.GetSort(realA)); sort != reals {
		t.Fatal("to-real", sort, reals)
	}
	integerR := f.pin(f.c.RealToInteger(r))
	if sort := f.pin(f.m.GetSort(integerR)); sort != ints {
		t.Fatal("to-int", sort, ints)
	}
}

func TestSMTContextArithmeticValidation(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	if _, err := f.c.Real(1, 0); err != ast.ErrASTDomain {
		t.Fatal("zero denominator", err)
	}
	if _, err := f.c.ArithmeticSub(); err != ast.ErrASTDomain {
		t.Fatal("empty sub", err)
	}
}
