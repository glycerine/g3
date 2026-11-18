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

// interval::negate builds a loose interval, and operator== compares
// tightness: asserting (bvule x 4) and then (not (bvule 5 x)) pushes an
// undo record that only loosens the bound (AUDIT tactic_bv_01-F7).
func TestBVBoundsNegatedBoundIsLoose(t *testing.T) {
	f := newBVBoundsFixture(t)
	ok, e := f.simp.AssertExpression(f.ule(t, f.x, f.num(t, 4)), false)
	sharingMust(t, e)
	if !ok || f.simp.ScopeLevel() != 1 {
		t.Fatal("first bound", ok, f.simp.ScopeLevel())
	}
	if !f.simp.bounds[f.x].tight {
		t.Fatal("asserted bound must be tight")
	}
	ok, e = f.simp.AssertExpression(f.ule(t, f.num(t, 5), f.x), true)
	sharingMust(t, e)
	if !ok || f.simp.ScopeLevel() != 2 {
		t.Fatal("negated bound must push an undo record", ok, f.simp.ScopeLevel())
	}
	if f.simp.bounds[f.x].tight {
		t.Fatal("intersection with a negated bound must be loose")
	}
	sharingMust(t, f.simp.Pop(1))
	if f.simp.ScopeLevel() != 1 || !f.simp.bounds[f.x].tight {
		t.Fatal("pop must restore the tight bound", f.simp.ScopeLevel())
	}
}
