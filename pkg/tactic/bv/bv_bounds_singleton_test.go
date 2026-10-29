package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVBoundsSourceSingletonReplacesVariable(t *testing.T) {
	f := newBVBoundsFixture(t)
	eq, e := f.m.MkEq(f.x, f.num(t, 7))
	sharingMust(t, e)
	ok, e := f.simp.AssertExpression(eq, false)
	sharingMust(t, e)
	if !ok {
		t.Fatal()
	}
	out, e := f.m.NewRef(ast.ASTHandle{})
	sharingMust(t, e)
	defer f.m.DelRef(&out)
	done, e := f.simp.Simplify(f.x, out)
	sharingMust(t, e)
	if !done {
		t.Fatal()
	}
	got, e := f.m.RefGet(out)
	sharingMust(t, e)
	n, _, yes, e := bvBoundsNumeral(f.m, got)
	sharingMust(t, e)
	if !yes || n.Int64() != 7 {
		t.Fatal(n, yes)
	}
}
