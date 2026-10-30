package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type fpaExponentSignificandFixture struct {
	*fpaNumeralPartsFixture
	integer, real, rounding ast.ASTHandle
}

func newFPAExponentSignificandFixture(t *testing.T) *fpaExponentSignificandFixture {
	t.Helper()
	base := newFPANumeralPartsFixture(t)
	f := &fpaExponentSignificandFixture{fpaNumeralPartsFixture: base}
	value, err := f.context.IntegerSort()
	f.integer = f.pin(t, value, err)
	value, err = f.context.RealSort()
	f.real = f.pin(t, value, err)
	value, err = f.context.FloatingPointRoundNearestEven()
	f.rounding = f.pin(t, value, err)
	return f
}

func (f *fpaExponentSignificandFixture) numeral(t *testing.T, text string, sort ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	value, err := f.context.Numeral(text, sort)
	return f.pin(t, value, err)
}

func (f *fpaExponentSignificandFixture) convert(t *testing.T, rounding, exponent, significand, target ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	value, err := f.context.FloatingPointFromExponentSignificand(rounding, exponent, significand, target)
	return f.pin(t, value, err)
}

func (f *fpaExponentSignificandFixture) rewrite(t *testing.T, value ast.ASTHandle) (ast.ASTHandle, bool) {
	t.Helper()
	declaration, err := f.manager.GetDecl(value)
	declaration = f.pin(t, declaration, err)
	arguments := [3]ast.ASTHandle{}
	for index := range arguments {
		arguments[index], err = f.manager.GetArg(value, uint32(index))
		if err != nil {
			t.Fatal(err)
		}
	}
	result, found, err := f.manager.RewriteFPAToFPExponentRationalNumerals(declaration, arguments[0], arguments[1], arguments[2])
	if err != nil {
		t.Fatal(err)
	}
	if found {
		result = f.pin(t, result, nil)
	}
	return result, found
}

func (f *fpaExponentSignificandFixture) requireParts(t *testing.T, value ast.ASTHandle, sign int, exponent, significand string) {
	t.Helper()
	actualSign, err := f.context.FloatingPointNumeralSign(value)
	if err != nil || actualSign != sign {
		t.Fatalf("sign %d, want %d: %v", actualSign, sign, err)
	}
	actualExponent, err := f.context.FloatingPointNumeralExponentString(value, false)
	if err != nil || actualExponent != exponent {
		t.Fatalf("exponent %q, want %q: %v", actualExponent, exponent, err)
	}
	actualSignificand, err := f.context.FloatingPointNumeralSignificandString(value)
	if err != nil || actualSignificand != significand {
		t.Fatalf("significand %q, want %q: %v", actualSignificand, significand, err)
	}
}
