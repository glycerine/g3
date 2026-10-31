package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExprQEArithmeticSearchRetestsPropagation(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	x := f.constant("search-arith-x", f.integer)
	y := f.constant("search-arith-y", f.integer)
	truth := f.own(m.MkTrue())
	lower := f.own(m.MkArithmeticGE(x, y))
	upperLimit := f.own(m.MkArithmeticAdd(y, f.number("1")))
	upper := f.own(m.MkArithmeticLE(x, upperLimit))
	formula := f.own(m.MkAnd(lower, upper))
	fmMust(t, solver.AssertFormula(formula))
	context, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	defer func() { fmMust(t, context.Close()) }()
	boolPlugin, err := NewBoolPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(boolPlugin))
	arithPlugin, err := NewArithmeticPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(arithPlugin))
	adapter, err := NewSolverNNFAdapter(context)
	fmMust(t, err)
	defer func() { fmMust(t, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, nnf.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	arithPlugin.util.theory = rw
	partitioner, err := NewConjunctionPartitioner(m)
	fmMust(t, err)
	defer func() { fmMust(t, partitioner.Close()) }()
	fmMust(t, partitioner.AddPlugin(boolPlugin))
	fmMust(t, partitioner.AddPlugin(arithPlugin))
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(x))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	fmMust(t, normalizeExprQETree(root, rw, nnf))
	result, complete, err := searchExprQEBoolTree(root, variables, context, boolPlugin, partitioner, solver, f.r.config, rw, nnf)
	fmMust(t, err)
	if !complete {
		t.Fatal("arithmetic search was incomplete")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	notValue := f.own(m.MkNot(value))
	check, err := k.solver()
	fmMust(t, err)
	fmMust(t, check.AssertFormula(notValue))
	status, err := check.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("the integer interval [y, y+1] should always be satisfiable")
	}
}
