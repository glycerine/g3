package qe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
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
	first := f.own(m.MkOr(f.own(m.MkEq(i, j)), f.own(m.MkEq(selectAt(b, i), one))))
	second := f.own(m.MkEq(selectAt(b, j), two))
	expected := f.own(m.MkAnd(f.own(m.MkAnd(originalRewritten, first)), second))
	got, present, e := ctx.CurrentFormula()
	fmMust(t, e)
	if !present || got != expected {
		t.Fatal("rewritten store equation", got, expected)
	}
	// The outer write wins when the indices coincide. The upstream
	// transformation incorrectly requires the inner value to remain visible.
	kernel := qsatKernelOwner(t, f)
	solver, e := kernel.solver()
	fmMust(t, e)
	fmMust(t, solver.AssertFormula(got))
	fmMust(t, solver.AssertFormula(f.own(m.MkEq(i, j))))
	fmMust(t, solver.AssertFormula(f.own(m.MkEq(selectAt(b, i), two))))
	status, e := solver.CheckSat(nil)
	fmMust(t, e)
	if status != lbool.True {
		t.Fatal("overwritten inner store made a satisfiable projection inconsistent", status)
	}
}

func TestArrayPluginStoreAllocationFrontiers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 1024 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			family, err := m.RegisterArrayPlugin()
			fmMust(t, err)
			sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer, f.integer}), f.integer))
			a, b := f.constant("budget-a", sort), f.constant("budget-b", sort)
			i, j := f.constant("budget-i", f.integer), f.constant("budget-j", f.integer)
			k, l := f.constant("budget-k", f.integer), f.constant("budget-l", f.integer)
			v, w := f.number("1"), f.number("2")
			inner := f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, j, v}))
			outer := f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{inner, k, l, w}))
			formula := f.own(m.MkEq(outer, b))
			ctx, err := NewSolverContext(m, []ast.ASTHandle{a})
			fmMust(t, err)
			plugin, err := NewArrayPlugin(ctx)
			fmMust(t, err)
			fmMust(t, ctx.AddPlugin(plugin))
			m.Budget().FailAfter(stop)
			solved, err := plugin.TryStoreEquality(outer, b, formula)
			m.Budget().FailAfter(-1)
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				fmMust(t, construction.Cleanup())
			}
			if err == nil {
				if !solved {
					t.Fatal("store equation was not solved")
				}
				success = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				var memout memory.Failure
				if !errors.As(err, &memout) || memout.Kind != memory.FailureMemout {
					t.Fatal(err)
				}
			}
			fmMust(t, ctx.Close())
		})
		if t.Failed() {
			t.Fatal("store elimination allocation frontier", stop)
		}
	}
	if !success {
		t.Fatal("no successful store elimination frontier")
	}
}
