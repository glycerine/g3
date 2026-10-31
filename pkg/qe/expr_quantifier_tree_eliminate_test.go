package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestExprQETreeEliminationCreatesNormalizedUnitBranch(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("eliminate-x", f.boolean)
	y := f.constant("eliminate-y", f.boolean)
	z := f.constant("eliminate-z", f.boolean)
	truth := f.own(m.MkTrue())
	vars, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, vars.Close()) }()
	fmMust(t, vars.add(y))
	fmMust(t, vars.add(x))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(truth))
	fmMust(t, vars.consume(root))
	fmMust(t, vars.add(z))
	ctx, err := NewSolverContext(m, []ast.ASTHandle{y, z})
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Close()) }()
	adapter, err := NewSolverNNFAdapter(ctx)
	fmMust(t, err)
	defer func() { fmMust(t, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, nnf.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	formula := f.own(m.MkImplies(z, y))
	child, err := eliminateExprQETreeVariable(root, 0, formula, z, vars, f.r.config, rw, nnf)
	fmMust(t, err)
	if !root.isUnit() || child.parent != root || len(child.variables) != 2 || len(vars.pending) != 0 {
		t.Fatal("incorrect unit branch state")
	}
	for i, want := range []ast.ASTHandle{y, z} {
		h, e := m.RefGet(child.variables[i])
		fmMust(t, e)
		if h != want {
			t.Fatal("child variable order", i)
		}
	}
	count, err := child.definitions.Size()
	fmMust(t, err)
	if count != 1 {
		t.Fatal("missing eliminated variable definition")
	}
	decl, err := m.GetDecl(x)
	fmMust(t, err)
	got, err := child.definitions.Variable(0)
	fmMust(t, err)
	if got != decl {
		t.Fatal("wrong definition variable")
	}
	negative, err := child.atoms.Negative(z)
	fmMust(t, err)
	positive, err := child.atoms.Positive(y)
	fmMust(t, err)
	if !negative || !positive {
		t.Fatal("child atom index not normalized")
	}
}
