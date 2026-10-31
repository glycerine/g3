package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestSolverContextOwnedVariableTransferAndSwap(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	ctx, e := NewSolverContext(m, nil)
	fmMust(t, e)
	makeOwned := func(name string) ast.ASTHandle {
		h, e := m.MkFreshConst(name, f.integer)
		fmMust(t, e)
		ref, e := m.NewRef(h)
		fmMust(t, e)
		fmMust(t, ctx.adoptVariable(h, &ref))
		if ref != (ast.ASTRef{}) {
			t.Fatal("owner not transferred")
		}
		return h
	}
	first, second := makeOwned("first"), makeOwned("second")
	truth := f.own(m.MkBoolVal(true))
	fmMust(t, ctx.EliminateVariable(0, truth, truth))
	vars, e := ctx.Variables()
	fmMust(t, e)
	if len(vars) != 1 || vars[0] != second {
		t.Fatal("swap did not move owner", vars)
	}
	_, e = m.GetKind(second)
	fmMust(t, e)
	if first == second {
		t.Fatal("fresh symbols collided")
	}
	fmMust(t, ctx.Close())
}
