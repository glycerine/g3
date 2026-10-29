package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVBoundsSourceAssertionIntersectionAndPop(t *testing.T) {
	f := newBVBoundsFixture(t)
	lo := f.ule(t, f.num(t, 4), f.x)
	hi := f.ule(t, f.x, f.num(t, 200))
	ok, e := f.simp.AssertExpression(lo, false)
	sharingMust(t, e)
	if !ok {
		t.Fatal()
	}
	ok, e = f.simp.AssertExpression(hi, false)
	sharingMust(t, e)
	if !ok || f.simp.ScopeLevel() != 2 {
		t.Fatal()
	}
	out, e := f.m.NewRef(ast.ASTHandle{})
	sharingMust(t, e)
	defer f.m.DelRef(&out)
	done, e := f.simp.Simplify(f.ule(t, f.x, f.num(t, 250)), out)
	sharingMust(t, e)
	if !done {
		t.Fatal("implied bound not simplified")
	}
	v, e := f.m.RefGet(out)
	sharingMust(t, e)
	if !f.m.IsTrue(v) {
		t.Fatal("not true")
	}
	sharingMust(t, f.simp.Pop(1))
	if f.simp.ScopeLevel() != 1 {
		t.Fatal()
	}
}
