package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExprQEBoolPopSolvesAndBlocksPath(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	p := f.constant("pop-p", f.boolean)
	q := f.constant("pop-q", f.boolean)
	truth := f.own(m.MkTrue())
	formula := f.own(m.MkAnd(p, q))
	fmMust(t, solver.AssertFormula(formula))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("pop fixture is not satisfiable")
	}
	context, err := NewSolverContext(m, []ast.ASTHandle{p, q})
	fmMust(t, err)
	defer func() { fmMust(t, context.Close()) }()
	plugin, err := NewBoolPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(plugin))
	adapter, err := NewSolverNNFAdapter(context)
	fmMust(t, err)
	defer func() { fmMust(t, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, nnf.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	partitioner, err := NewConjunctionPartitioner(m)
	fmMust(t, err)
	defer func() { fmMust(t, partitioner.Close()) }()
	fmMust(t, partitioner.AddPlugin(plugin))
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(q))
	fmMust(t, variables.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	root.atoms, err = NewBoolAtomIndex(m, []ast.ASTHandle{p, q}, nil)
	fmMust(t, err)
	var free []ast.ASTRef
	leaf, outcome, err := popExprQEBoolTree(root, variables, context, plugin, partitioner, solver, nil, nil, f.r.config, rw, nnf, &free, false)
	fmMust(t, err)
	if outcome != exprQEPopCompleted || len(leaf.variables) != 0 || len(free) != 0 {
		t.Fatal("Boolean pop did not complete both units")
	}
	h, err := m.RefGet(leaf.formula)
	fmMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("eliminated leaf is not true")
	}
	status, err = solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("completed pop did not block the explored path")
	}
}
