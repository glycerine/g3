package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArrayPluginNestedStoreElimination(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	family, e := m.RegisterArrayPlugin()
	fmMust(t, e)
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a, b := f.constant("store-a", sort), f.constant("store-b", sort)
	i, j := f.constant("i", f.integer), f.constant("j", f.integer)
	one, two := f.number("1"), f.number("2")
	store := func(base, index, value ast.ASTHandle) ast.ASTHandle {
		return f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, index, value}))
	}
	selectAt := func(base, index ast.ASTHandle) ast.ASTHandle {
		return f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{base, index}))
	}
	inner := store(a, i, one)
	outer := store(inner, j, two)
	eq := f.own(m.MkEq(outer, b))
	guard := f.own(m.MkEq(a, b))
	formula := f.own(m.MkAnd(eq, guard))
	ctx, e := NewSolverContext(m, []ast.ASTHandle{a})
	fmMust(t, e)
	p, e := NewArrayPlugin(ctx)
	fmMust(t, e)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	solved, e := p.TryStoreEquality(outer, a, formula)
	fmMust(t, e)
	if solved {
		t.Fatal("cyclic rhs accepted")
	}
	n, e := ctx.NumVariables()
	fmMust(t, e)
	if n != 1 {
		t.Fatal("failure mutated variables")
	}
	solved, e = p.TryStoreEquality(outer, b, formula)
	fmMust(t, e)
	if !solved {
		t.Fatal("store chain not solved")
	}
	vars, e := ctx.Variables()
	fmMust(t, e)
	if len(vars) != 2 || vars[0] == vars[1] || vars[0] == a || vars[1] == a {
		t.Fatal("fresh variables", vars)
	}
	outerFresh, innerFresh := vars[0], vars[1]
	for _, h := range vars {
		s, e := m.GetSort(h)
		fmMust(t, e)
		if s != f.integer {
			t.Fatal("fresh value sort")
		}
	}
	repl := store(store(b, i, innerFresh), j, outerFresh)
	originalRewritten := f.own(m.MkAnd(f.own(m.MkEq(store(store(repl, i, one), j, two), b)), f.own(m.MkEq(repl, b))))
	first := f.own(m.MkEq(selectAt(b, i), one))
	realInner := store(b, i, one)
	second := f.own(m.MkEq(selectAt(realInner, j), two))
	expected := f.own(m.MkAnd(f.own(m.MkAnd(originalRewritten, first)), second))
	got, present, e := ctx.CurrentFormula()
	fmMust(t, e)
	if !present || got != expected {
		t.Fatal("rewritten store equation", got, expected)
	}
}
