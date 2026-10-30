package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBVBoundsSimplifyTightNegation(t *testing.T) {
	f := newBVBoundsFixture(t)
	ok, err := f.simp.AssertExpression(f.ule(t, f.num(t, 20), f.x), false)
	sharingMust(t, err)
	if !ok {
		t.Fatal("context bound rejected")
	}
	upper := f.ule(t, f.x, f.num(t, 10))
	notUpper, err := f.m.MkNot(upper)
	sharingMust(t, err)
	out, err := f.m.NewRef(ast.ASTHandle{})
	sharingMust(t, err)
	defer f.m.DelRef(&out)
	done, err := f.simp.Simplify(notUpper, out)
	sharingMust(t, err)
	value, err := f.m.RefGet(out)
	sharingMust(t, err)
	if !done || !f.m.IsTrue(value) {
		t.Fatal("tight negated bound was not proved true")
	}
}

func TestBVBoundsSimplifyDoubleNegationParity(t *testing.T) {
	f := newBVBoundsFixture(t)
	ok, err := f.simp.AssertExpression(f.ule(t, f.x, f.num(t, 10)), false)
	sharingMust(t, err)
	if !ok {
		t.Fatal("context bound rejected")
	}
	bound := f.ule(t, f.x, f.num(t, 20))
	one, err := f.m.MkNot(bound)
	sharingMust(t, err)
	two, err := f.m.MkNot(one)
	sharingMust(t, err)
	out, err := f.m.NewRef(ast.ASTHandle{})
	sharingMust(t, err)
	defer f.m.DelRef(&out)
	done, err := f.simp.Simplify(two, out)
	sharingMust(t, err)
	value, err := f.m.RefGet(out)
	sharingMust(t, err)
	if !done || !f.m.IsTrue(value) {
		t.Fatal("even not parity was not retained")
	}
}
