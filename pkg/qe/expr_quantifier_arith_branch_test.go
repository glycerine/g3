package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestExprQEArithmeticBranchTransfersConstraintsAndDefinition(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "5")
	m := f.r.manager
	k := qsatKernelOwner(t, f.fmFixture)
	solver, err := k.solver()
	fmMust(t, err)
	zero := f.number("0")
	minus := f.number("-1")
	lower := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticMul(minus, f.x)), zero))
	upper := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(f.x, f.number("-5"))), zero))
	formula := f.own(m.MkAnd(lower, upper))
	ok, err := f.plugin.Prepare(f.contains, formula, []ast.ASTHandle{lower, upper}, nil)
	fmMust(t, err)
	if !ok {
		t.Fatal("bound preparation")
	}
	vars, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, vars.Close()) }()
	fmMust(t, vars.add(f.x))
	truth := f.own(m.MkTrue())
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(formula))
	fmMust(t, vars.consume(root))
	fmMust(t, root.selectVariable(f.x, big.NewRat(2, 1)))
	ctx, err := NewSolverContext(m, nil)
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
	child, status, err := stepExprQEScalarBranch(root, vars, f.plugin, big.NewRat(1, 1), f.r.config, rw, nnf, solver, true)
	fmMust(t, err)
	if status != exprQENeedPropagation || child.formula != (ast.ASTRef{}) {
		t.Fatal("arithmetic branch propagation")
	}
	assertions, err := solver.NumAssertions()
	fmMust(t, err)
	if assertions == 0 {
		t.Fatal("arithmetic constraints were not asserted")
	}
	child, status, err = stepExprQEScalarBranch(root, vars, f.plugin, big.NewRat(1, 1), f.r.config, rw, nnf, solver, true)
	fmMust(t, err)
	if status != exprQEChooseVariable {
		t.Fatal("arithmetic branch substitution")
	}
	result, err := m.RefGet(child.formula)
	fmMust(t, err)
	contains, err := f.contains.Check(result)
	fmMust(t, err)
	if contains {
		t.Fatal("arithmetic variable survived substitution")
	}
	n, err := child.definitions.Size()
	fmMust(t, err)
	if n != 1 {
		t.Fatal("arithmetic definition missing")
	}
	constraints, err := f.plugin.TakeConstraints()
	fmMust(t, err)
	if len(constraints) != 0 {
		t.Fatal("constraints not transferred")
	}
}

func TestExprQEArithmeticBranchTransfersFreshVariables(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "5")
	m := f.r.manager
	k := qsatKernelOwner(t, f.fmFixture)
	solver, err := k.solver()
	fmMust(t, err)
	y := f.constant("cooper-free-y", f.integer)
	twoX := f.own(m.MkArithmeticMul(f.number("2"), f.x))
	polynomial := f.own(m.MkArithmeticAdd(twoX, y))
	mod := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{polynomial, f.number("7")}))
	formula := f.own(m.MkEq(mod, f.number("0")))
	ok, err := f.plugin.Prepare(f.contains, formula, []ast.ASTHandle{formula}, nil)
	fmMust(t, err)
	if !ok {
		t.Fatal("modular bound preparation")
	}
	vars, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, vars.Close()) }()
	fmMust(t, vars.add(f.x))
	truth := f.own(m.MkTrue())
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(formula))
	fmMust(t, vars.consume(root))
	fmMust(t, root.selectVariable(f.x, big.NewRat(2, 1)))
	ctx, err := NewSolverContext(m, nil)
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
	child, status, err := stepExprQEScalarBranch(root, vars, f.plugin, big.NewRat(0, 1), f.r.config, rw, nnf, solver, false)
	fmMust(t, err)
	if status != exprQENeedPropagation || len(child.variables) != 1 || len(vars.pending) != 0 {
		t.Fatal("fresh Cooper variable was not added to child")
	}
	fresh, err := m.RefGet(child.variables[0])
	fmMust(t, err)
	if fresh == f.x || fresh == y {
		t.Fatal("expected a fresh variable")
	}
	assertions, err := solver.NumAssertions()
	fmMust(t, err)
	if assertions < 4 {
		t.Fatalf("got %d Cooper constraints, want at least four", assertions)
	}
}
