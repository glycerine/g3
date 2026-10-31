package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestExprQEBoolSolvePrefersUnitConjunctOrder(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	p := f.constant("solve-p", f.boolean)
	q := f.constant("solve-q", f.boolean)
	truth := f.own(m.MkTrue())
	ctx, err := NewSolverContext(m, []ast.ASTHandle{p, q})
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Close()) }()
	plugin, err := NewBoolPlugin(ctx)
	fmMust(t, err)
	defer func() { fmMust(t, plugin.Close()) }()
	vars, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, vars.Close()) }()
	fmMust(t, vars.add(q))
	fmMust(t, vars.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	formula := f.own(m.MkAnd(p, q))
	fmMust(t, root.initialize(formula))
	fmMust(t, vars.consume(root))
	root.atoms, err = NewBoolAtomIndex(m, []ast.ASTHandle{p, q}, nil)
	fmMust(t, err)
	conjs, err := NewConjunctionEnum(m, formula)
	fmMust(t, err)
	defer func() { fmMust(t, conjs.Close()) }()
	first, err := conjs.At(0)
	fmMust(t, err)
	wantIndex, err := root.variableIndex(first)
	fmMust(t, err)
	if wantIndex != 1 {
		t.Fatal("test did not distinguish conjunct and variable order")
	}
	index, result, found, err := chooseExprQEBoolElimination(root, vars, plugin, conjs)
	fmMust(t, err)
	if !found || int(index) != wantIndex {
		t.Fatal("first unit conjunct was not selected", index)
	}
	defer func() { fmMust(t, result.Close()) }()
	definition, err := m.RefGet(result.Definition)
	fmMust(t, err)
	if !m.IsTrue(definition) {
		t.Fatal("unit definition")
	}
	rewritten, err := m.RefGet(result.Formula)
	fmMust(t, err)
	contains, err := ast.NewContainsApp(m, first)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	yes, err := contains.Check(rewritten)
	fmMust(t, err)
	if yes {
		t.Fatal("eliminated variable remains")
	}
	polarized := f.own(m.MkOr(p, q))
	fmMust(t, root.initialize(polarized))
	polarConjs, err := NewConjunctionEnum(m, polarized)
	fmMust(t, err)
	defer func() { fmMust(t, polarConjs.Close()) }()
	index, polarityResult, found, err := chooseExprQEBoolElimination(root, vars, plugin, polarConjs)
	fmMust(t, err)
	if !found || index != 0 {
		t.Fatal("first polarized variable was not selected")
	}
	defer func() { fmMust(t, polarityResult.Close()) }()
	definition, err = m.RefGet(polarityResult.Definition)
	fmMust(t, err)
	if !m.IsTrue(definition) {
		t.Fatal("positive polarity definition")
	}
}

func TestExprQEBoolSolveCreatesUnitBranch(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	p := f.constant("solve-unit-p", f.boolean)
	q := f.constant("solve-unit-q", f.boolean)
	truth := f.own(m.MkTrue())
	ctx, err := NewSolverContext(m, []ast.ASTHandle{p, q})
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Close()) }()
	plugin, err := NewBoolPlugin(ctx)
	fmMust(t, err)
	defer func() { fmMust(t, plugin.Close()) }()
	adapter, err := NewSolverNNFAdapter(ctx)
	fmMust(t, err)
	defer func() { fmMust(t, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, nnf.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(q))
	fmMust(t, variables.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	formula := f.own(m.MkAnd(p, q))
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	root.atoms, err = NewBoolAtomIndex(m, []ast.ASTHandle{p, q}, nil)
	fmMust(t, err)
	conjs, err := NewConjunctionEnum(m, formula)
	fmMust(t, err)
	defer func() { fmMust(t, conjs.Close()) }()
	child, solved, err := solveExprQEBool(root, variables, plugin, conjs, f.r.config, rw, nnf)
	fmMust(t, err)
	if !solved || !root.isUnit() || child == nil || len(child.variables) != 1 {
		t.Fatal("Boolean solve did not publish unit branch")
	}
	remaining, err := m.RefGet(child.variables[0])
	fmMust(t, err)
	h, err := m.RefGet(child.formula)
	fmMust(t, err)
	if h != remaining {
		t.Fatal("unit substitution did not simplify to remaining variable")
	}
	count, err := child.definitions.Size()
	fmMust(t, err)
	if count != 1 {
		t.Fatal("unit definition missing")
	}
}
