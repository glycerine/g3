package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestArrayPluginSolveExtractionFallbackRejectsSubtraction(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x, y := f.constant("fallback-x", f.integer), f.constant("fallback-y", f.integer)
	le := f.own(m.MkArithmeticLE(x, y))
	ge := f.own(m.MkArithmeticGE(x, y))
	formula := f.own(m.MkAnd(le, ge))
	ctx, e := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, e)
	p, e := NewArrayPlugin(ctx)
	fmMust(t, e)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	conjs, e := NewConjunctionEnum(m, formula)
	fmMust(t, e)
	defer func() { fmMust(t, conjs.Close()) }()
	equalities, e := conjs.ExtractEqualities()
	fmMust(t, e)
	if len(equalities) != 1 {
		t.Fatal("expected equality from opposite bounds", len(equalities))
	}
	term, e := m.RefGet(equalities[0])
	fmMust(t, e)
	op, e := m.GetDeclKind(term)
	fmMust(t, e)
	if op != 7 {
		t.Fatal("extracted term is not subtraction", op)
	}
	fmMust(t, m.DelRef(&equalities[0]))
	solved, e := p.Solve(conjs, formula)
	fmMust(t, e)
	if solved {
		t.Fatal("source add-only fallback unexpectedly solved subtraction")
	}
	n, e := ctx.NumVariables()
	fmMust(t, e)
	if n != 1 {
		t.Fatal("variable changed", n)
	}
	_, present, e := ctx.CurrentFormula()
	fmMust(t, e)
	if present {
		t.Fatal("formula published despite no solution")
	}
}

func TestArrayPluginSolveDispatchesExplicitEquality(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	_, e := m.RegisterArrayPlugin()
	fmMust(t, e)
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a, b := f.constant("solve-a", sort), f.constant("solve-b", sort)
	eq := f.own(m.MkEq(a, b))
	ctx, e := NewSolverContext(m, []ast.ASTHandle{a})
	fmMust(t, e)
	p, e := NewArrayPlugin(ctx)
	fmMust(t, e)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	conjs, e := NewConjunctionEnum(m, eq)
	fmMust(t, e)
	defer func() { fmMust(t, conjs.Close()) }()
	solved, e := p.Solve(conjs, eq)
	fmMust(t, e)
	if !solved {
		t.Fatal("explicit equation not solved")
	}
	n, e := ctx.NumVariables()
	fmMust(t, e)
	if n != 0 {
		t.Fatal(n)
	}
}

func TestArrayPluginSolveSkipsQuantifiedConjunct(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	truth := f.own(m.MkBoolVal(true))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: truth}))
	x := f.constant("quantified-variable", f.integer)
	ctx, e := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, e)
	p, e := NewArrayPlugin(ctx)
	fmMust(t, e)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	conjs, e := NewConjunctionEnum(m, q)
	fmMust(t, e)
	defer func() { fmMust(t, conjs.Close()) }()
	solved, e := p.Solve(conjs, q)
	fmMust(t, e)
	if solved {
		t.Fatal("quantifier treated as equality")
	}
}
