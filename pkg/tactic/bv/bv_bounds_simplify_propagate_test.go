package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBVBoundsSimplifyNegatedIntersectionPropagatesEquality(t *testing.T) {
	f := newBVBoundsFixture(t)
	f.simp.propagateEq = true
	for _, bound := range []ast.ASTHandle{
		f.ule(t, f.num(t, 10), f.x),
		f.ule(t, f.x, f.num(t, 11)),
	} {
		ok, err := f.simp.AssertExpression(bound, false)
		sharingMust(t, err)
		if !ok {
			t.Fatal("context bound rejected")
		}
	}
	upper := f.ule(t, f.x, f.num(t, 10))
	query, err := f.m.MkNot(upper)
	sharingMust(t, err)
	out, err := f.m.NewRef(ast.ASTHandle{})
	sharingMust(t, err)
	defer f.m.DelRef(&out)
	done, err := f.simp.Simplify(query, out)
	sharingMust(t, err)
	value, err := f.m.RefGet(out)
	sharingMust(t, err)
	if !done {
		t.Fatal("singleton intersection was not propagated")
	}
	rhs, err := f.m.GetArg(value, 1)
	sharingMust(t, err)
	n, width, numeral, err := bvBoundsNumeral(f.m, rhs)
	sharingMust(t, err)
	if !numeral || width != 8 || n.Int64() != 11 {
		t.Fatalf("propagated equality rhs = %v/%d/%v", n, width, numeral)
	}
}
