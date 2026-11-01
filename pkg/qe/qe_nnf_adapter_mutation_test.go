package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSolverNNFAdapterTracksAddedAndRemovedVariables(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x := f.constant("nnf-mutation-x", f.integer)
	y := f.constant("nnf-mutation-y", f.integer)
	z := f.constant("nnf-mutation-z", f.integer)
	context, err := NewSolverContext(m, []ast.ASTHandle{x, y})
	fmMust(t, err)
	defer func() { fmMust(t, context.Close()) }()
	adapter, err := NewSolverNNFAdapter(context)
	fmMust(t, err)
	defer func() { fmMust(t, adapter.Close()) }()
	for _, tc := range []struct {
		term ast.ASTHandle
		want bool
	}{{x, true}, {y, true}, {z, false}} {
		got, e := adapter.IsRelevant(tc.term)
		fmMust(t, e)
		if got != tc.want {
			t.Fatal("initial relevance", got, tc.want)
		}
	}
	fmMust(t, context.AddVariable(z))
	relevant, err := adapter.IsRelevant(z)
	fmMust(t, err)
	if !relevant {
		t.Fatal("added variable not relevant")
	}
	truth := f.own(m.MkTrue())
	fmMust(t, context.EliminateVariable(0, truth, ast.ASTHandle{}))
	for _, tc := range []struct {
		term ast.ASTHandle
		want bool
	}{{x, false}, {y, true}, {z, true}} {
		got, e := adapter.IsRelevant(tc.term)
		fmMust(t, e)
		if got != tc.want {
			t.Fatal("relevance after swap-removal", got, tc.want)
		}
	}
}
