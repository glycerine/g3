package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBVBoundsDoesNotSimplifyNegatedExtractFromBaseBound(t *testing.T) {
	f := newBVBoundsFixture(t)
	ok, err := f.simp.AssertExpression(f.ule(t, f.x, f.num(t, 15)), false)
	sharingMust(t, err)
	if !ok {
		t.Fatal("context bound rejected")
	}
	top, err := f.m.MkBitVectorExtract(7, 4, f.x)
	sharingMust(t, err)
	zero, err := f.m.BitVectorNumeralUnsigned(0, 4)
	sharingMust(t, err)
	eq, err := f.m.MkEq(zero, top)
	sharingMust(t, err)
	query, err := f.m.MkNot(eq)
	sharingMust(t, err)
	out, err := f.m.NewRef(ast.ASTHandle{})
	sharingMust(t, err)
	defer f.m.DelRef(&out)
	done, err := f.simp.Simplify(query, out)
	sharingMust(t, err)
	if done {
		t.Fatal("a bound on the base vector must not be applied to a distinct extract term")
	}
}
