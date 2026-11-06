package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestSMTContextBitVectorOverflowPredicates(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.BitVectorSort(8))
	a := f.pin(f.c.BitVectorValue(100, sort))
	b := f.pin(f.c.BitVectorValue(30, sort))
	boolean := f.pin(f.m.MkBoolSort())
	terms := []ast.ASTHandle{
		f.pin(f.c.BitVectorAddNoOverflow(a, b, false)), f.pin(f.c.BitVectorAddNoOverflow(a, b, true)), f.pin(f.c.BitVectorAddNoUnderflow(a, b)),
		f.pin(f.c.BitVectorSubNoOverflow(a, b)), f.pin(f.c.BitVectorSubNoUnderflow(a, b, false)), f.pin(f.c.BitVectorSubNoUnderflow(a, b, true)),
		f.pin(f.c.BitVectorMulNoOverflow(a, b, false)), f.pin(f.c.BitVectorMulNoOverflow(a, b, true)), f.pin(f.c.BitVectorMulNoUnderflow(a, b)),
		f.pin(f.c.BitVectorNegNoOverflow(a)), f.pin(f.c.BitVectorSDivNoOverflow(a, b)),
	}
	for i, term := range terms {
		if sort := f.pin(f.m.GetSort(term)); sort != boolean {
			t.Fatal("predicate sort", i, sort, boolean)
		}
	}
}

func TestSMTContextBitVectorExtrema(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.BitVectorSort(8))
	min := f.pin(f.c.BitVectorSignedMin(sort))
	if kind, err := f.m.GetDeclKind(min); err != nil || kind != 45 {
		t.Fatal("signed min", kind, err)
	}
	max := f.pin(f.c.BitVectorSignedMax(sort))
	if kind, err := f.m.GetDeclKind(max); err != nil || kind != 32 {
		t.Fatal("signed max", kind, err)
	}
	unsigned := f.pin(f.c.BitVectorUnsignedMax(sort))
	text, err := f.c.NumeralString(unsigned)
	if err != nil || text != "255" {
		t.Fatal("unsigned max", text, err)
	}
}

// api_bv.cpp constructs these atoms directly with MK_BINARY_BODY. Unlike
// bv_util's convenience functions, the C API preserves the operand order.
// Reversing just the overflow atom destroys circuit sharing with a widened
// product and makes the symbolic equivalence proof needlessly expensive.
func TestSMTContextBitVectorMultiplyBoundsPreserveOperandOrder(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.BitVectorSort(16))
	x := f.pin(f.c.Constant("x", sort))
	y := f.pin(f.c.Constant("y", sort))
	for _, pair := range [][2]ast.ASTHandle{{x, y}, {y, x}} {
		for _, tc := range []struct {
			name string
			kind int32
			make func(ast.ASTHandle, ast.ASTHandle) (ast.ASTHandle, error)
		}{
			{"signed-overflow", 53, func(a, b ast.ASTHandle) (ast.ASTHandle, error) { return f.c.BitVectorMulNoOverflow(a, b, true) }},
			{"unsigned-overflow", 52, func(a, b ast.ASTHandle) (ast.ASTHandle, error) { return f.c.BitVectorMulNoOverflow(a, b, false) }},
			{"signed-underflow", 54, f.c.BitVectorMulNoUnderflow},
		} {
			h := f.pin(tc.make(pair[0], pair[1]))
			if kind, err := f.m.GetDeclKind(h); err != nil || kind != tc.kind {
				t.Fatalf("%s kind %d: %v", tc.name, kind, err)
			}
			for i, want := range pair {
				got, err := f.m.GetArg(h, uint32(i))
				if err != nil || got != want {
					t.Fatalf("%s operand %d: got %v, want %v: %v", tc.name, i, got, want, err)
				}
			}
		}
	}
}
