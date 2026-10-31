package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArrayPluginSelectEqualityIntroducesFreshArray(t *testing.T) {
	for _, dimensions := range []int{1, 2} {
		t.Run(string(rune('0'+dimensions)), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			family, e := m.RegisterArrayPlugin()
			fmMust(t, e)
			domains := make([]ast.ASTHandle, dimensions)
			for i := range domains {
				domains[i] = f.integer
			}
			sort := f.own(m.MkArraySort(ast.SliceArguments(domains), f.integer))
			a := f.constant("select-a", sort)
			other := f.constant("other", sort)
			i := f.constant("i", f.integer)
			j := f.constant("j", f.integer)
			value := f.number("3")
			indices := []ast.ASTHandle{i}
			if dimensions == 2 {
				indices = append(indices, j)
			}
			args := append([]ast.ASTHandle{a}, indices...)
			selected := f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, args))
			eq := f.own(m.MkEq(selected, value))
			same := f.own(m.MkEq(a, other))
			formula := f.own(m.MkAnd(eq, same))
			ctx, e := NewSolverContext(m, []ast.ASTHandle{a})
			fmMust(t, e)
			p, e := NewArrayPlugin(ctx)
			fmMust(t, e)
			fmMust(t, ctx.AddPlugin(p))
			defer func() { fmMust(t, ctx.Close()) }()
			// The variable occurring in rhs invalidates the select equation.
			solved, e := p.TrySelectEquality(selected, selected, formula)
			fmMust(t, e)
			if solved {
				t.Fatal("cyclic select accepted")
			}
			solved, e = p.TrySelectEquality(selected, value, formula)
			fmMust(t, e)
			if !solved {
				t.Fatal("select equation not solved")
			}
			variables, e := ctx.Variables()
			fmMust(t, e)
			if len(variables) != 1 || variables[0] == a {
				t.Fatal("fresh variable", variables)
			}
			fresh := variables[0]
			freshSort, e := m.GetSort(fresh)
			fmMust(t, e)
			if freshSort != sort {
				t.Fatal("fresh sort")
			}
			storeArgs := append([]ast.ASTHandle{fresh}, indices...)
			storeArgs = append(storeArgs, value)
			store := f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, storeArgs))
			selectStoreArgs := append([]ast.ASTHandle{store}, indices...)
			selectStore := f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, selectStoreArgs))
			expectedEq := f.own(m.MkEq(selectStore, value))
			expectedSame := f.own(m.MkEq(store, other))
			expected := f.own(m.MkAnd(expectedEq, expectedSame))
			result, present, e := ctx.CurrentFormula()
			fmMust(t, e)
			if !present || result != expected {
				t.Fatal("rewritten formula", result, expected)
			}
		})
	}
}

func TestArrayPluginSelectFreshVariableSurvivesAbsentFormula(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	family, e := m.RegisterArrayPlugin()
	fmMust(t, e)
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("unused-a", sort)
	i := f.number("0")
	value := f.number("1")
	selected := f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, i}))
	truth := f.own(m.MkBoolVal(true))
	ctx, e := NewSolverContext(m, []ast.ASTHandle{a})
	fmMust(t, e)
	p, e := NewArrayPlugin(ctx)
	fmMust(t, e)
	fmMust(t, ctx.AddPlugin(p))
	solved, e := p.TrySelectEquality(selected, value, truth)
	fmMust(t, e)
	if !solved {
		t.Fatal("select equation not solved")
	}
	variables, e := ctx.Variables()
	fmMust(t, e)
	if len(variables) != 1 {
		t.Fatal(variables)
	}
	_, e = m.GetKind(variables[0])
	fmMust(t, e)
	// The context owns the new variable even after the plugin is closed.
	fmMust(t, p.Close())
	_, e = m.GetKind(variables[0])
	fmMust(t, e)
	contains, e := ctx.Contains(0)
	fmMust(t, e)
	found, e := contains.Check(variables[0])
	fmMust(t, e)
	if !found {
		t.Fatal("fresh context variable lost its owner")
	}
	fmMust(t, ctx.Close())
}
