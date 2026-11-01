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
	result, complete, err := searchExprQEScalarTree(root, variables, context, boolPlugin, partitioner, solver, f.r.config, rw, nnf)
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

func TestExprQEArithmeticSearchWithBoundedCooperResidue(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("search-cooper-x", f.integer)
	y := f.constant("search-cooper-y", f.integer)
	twoX := f.own(m.MkArithmeticMul(f.number("2"), x))
	polynomial := f.own(m.MkArithmeticAdd(twoX, y))
	mod := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{polynomial, f.number("7")}))
	body := f.own(m.MkEq(mod, f.number("0")))
	bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{x}, body)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&bound)) }()
	quantifier, err := m.RefGet(bound)
	fmMust(t, err)
	result, complete, err := eliminateQuantifierArithmeticSearch(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("Cooper search did not finish")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	h, err := m.RefGet(result)
	fmMust(t, err)
	notResult := f.own(m.MkNot(h))
	kernel := qsatKernelOwner(t, f)
	solver, err := kernel.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(notResult))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("every integer has a solution modulo seven", status)
	}
}

func TestExprQENestedModuloProjection(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("search-nested-mod-x", f.integer)
	y := f.constant("search-nested-mod-y", f.integer)
	twoX := f.own(m.MkArithmeticMul(f.number("2"), x))
	sum := f.own(m.MkArithmeticAdd(twoX, y))
	mod := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{sum, f.number("4")}))
	body := f.own(m.MkEq(mod, f.number("1")))
	bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{x}, body)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&bound)) }()
	quantifier, err := m.RefGet(bound)
	fmMust(t, err)
	result, complete, err := eliminateQuantifierArithmeticSearch(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("nested modulo search did not finish")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	h, err := m.RefGet(result)
	fmMust(t, err)
	ymod := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{y, f.number("2")}))
	want := f.own(m.MkEq(ymod, f.number("1")))
	different := f.own(m.MkNot(f.own(m.MkEq(h, want))))
	kernel := qsatKernelOwner(t, f)
	solver, err := kernel.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("nested modulo projection differs from oddness", status)
	}
}
