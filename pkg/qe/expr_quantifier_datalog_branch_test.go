package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestExprQEDatalogBranchPropagatesAndSubstitutes(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	kernel := qsatKernelOwner(t, f)
	solver, err := kernel.solver()
	fmMust(t, err)
	name, err := m.Symbols.Intern("branch-finite-domain")
	fmMust(t, err)
	sort := f.own(m.MkDatalogFiniteSort(name, 4))
	x, y := f.constant("branch-finite-x", sort), f.constant("branch-finite-y", sort)
	formula := f.own(m.MkEq(x, y))
	context, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	defer func() { fmMust(t, context.Close()) }()
	plugin, err := NewDatalogPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(plugin))
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	ok, err := plugin.Prepare(contains, formula, []ast.ASTHandle{formula}, nil)
	fmMust(t, err)
	if !ok {
		t.Fatal("finite-domain preparation failed")
	}
	adapter, err := NewSolverNNFAdapter(context)
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
	fmMust(t, variables.add(x))
	truth := f.own(m.MkTrue())
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	fmMust(t, root.selectVariable(x, big.NewRat(2, 1)))
	branch := big.NewRat(0, 1)
	child, status, err := stepExprQEScalarBranch(root, variables, plugin, branch, f.r.config, rw, nnf, solver, true)
	fmMust(t, err)
	if status != exprQENeedPropagation || child.formula != (ast.ASTRef{}) {
		t.Fatal("branch did not request propagation")
	}
	assertions, err := solver.NumAssertions()
	fmMust(t, err)
	if assertions == 0 {
		t.Fatal("finite-domain constraint was not asserted")
	}
	child, status, err = stepExprQEScalarBranch(root, variables, plugin, branch, f.r.config, rw, nnf, solver, true)
	fmMust(t, err)
	if status != exprQEChooseVariable {
		t.Fatal("branch did not substitute")
	}
	h, err := m.RefGet(child.formula)
	fmMust(t, err)
	escaped, err := contains.Check(h)
	fmMust(t, err)
	if escaped {
		t.Fatal("selected finite-domain variable survived")
	}
	n, err := child.definitions.Size()
	fmMust(t, err)
	if n != 0 {
		t.Fatal("finite-domain branch unexpectedly defined the variable")
	}
}
