package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArrayPluginExplicitSolveUsesSourceSidePrecedence(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	family, e := m.RegisterArrayPlugin()
	fmMust(t, e)
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a, b := f.constant("precedence-a", sort), f.constant("precedence-b", sort)
	i := f.constant("i", f.integer)
	v := f.number("1")
	stored := f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, v}))
	equation := f.own(m.MkEq(stored, b))
	ctx, e := NewSolverContext(m, []ast.ASTHandle{a, b})
	fmMust(t, e)
	p, e := NewArrayPlugin(ctx)
	fmMust(t, e)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	conjs, e := NewConjunctionEnum(m, equation)
	fmMust(t, e)
	defer func() { fmMust(t, conjs.Close()) }()
	solved, e := p.SolveExplicitEqualities(conjs, equation)
	fmMust(t, e)
	if !solved {
		t.Fatal("store equality was not solved")
	}
	vars, e := ctx.Variables()
	fmMust(t, e)
	if len(vars) != 2 {
		t.Fatal(vars)
	}
	// The first side is a store on a, so the source removes a before considering
	// the direct equality b = store(a,...). The latter would remove b instead.
	if vars[1] != b || vars[0] == a || vars[0] == b {
		t.Fatalf("wrong side eliminated: %v", vars)
	}
}

func TestArrayPluginExplicitSolveSkipsNonEqualities(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	family, e := m.RegisterArrayPlugin()
	fmMust(t, e)
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a, b := f.constant("solve-a", sort), f.constant("solve-b", sort)
	i := f.number("0")
	v := f.number("7")
	sel := f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, i}))
	eq := f.own(m.MkEq(sel, v))
	negative := f.own(m.MkNot(eq))
	ctx, e := NewSolverContext(m, []ast.ASTHandle{a})
	fmMust(t, e)
	p, e := NewArrayPlugin(ctx)
	fmMust(t, e)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	conjs, e := NewConjunctionEnum(m, negative)
	fmMust(t, e)
	solved, e := p.SolveExplicitEqualities(conjs, negative)
	fmMust(t, e)
	fmMust(t, conjs.Close())
	if solved {
		t.Fatal("negated equality treated as positive")
	}
	reverse := f.own(m.MkEq(b, a))
	formula := f.own(m.MkAnd(negative, reverse))
	conjs, e = NewConjunctionEnum(m, formula)
	fmMust(t, e)
	defer func() { fmMust(t, conjs.Close()) }()
	solved, e = p.SolveExplicitEqualities(conjs, formula)
	fmMust(t, e)
	if !solved {
		t.Fatal("second conjunct not solved")
	}
	count, e := ctx.NumVariables()
	fmMust(t, e)
	if count != 0 {
		t.Fatal("variable remains")
	}
}

func TestArrayPluginExplicitSolveSelectInEitherOrientation(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "forward", true: "reverse"}[reverse], func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			family, e := m.RegisterArrayPlugin()
			fmMust(t, e)
			sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
			a := f.constant("select-a", sort)
			i, v := f.number("0"), f.number("9")
			sel := f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, i}))
			var equation ast.ASTHandle
			if reverse {
				equation = f.own(m.MkEq(v, sel))
			} else {
				equation = f.own(m.MkEq(sel, v))
			}
			ctx, e := NewSolverContext(m, []ast.ASTHandle{a})
			fmMust(t, e)
			p, e := NewArrayPlugin(ctx)
			fmMust(t, e)
			fmMust(t, ctx.AddPlugin(p))
			defer func() { fmMust(t, ctx.Close()) }()
			conjs, e := NewConjunctionEnum(m, equation)
			fmMust(t, e)
			defer func() { fmMust(t, conjs.Close()) }()
			solved, e := p.SolveExplicitEqualities(conjs, equation)
			fmMust(t, e)
			if !solved {
				t.Fatal("select was not solved")
			}
			vars, e := ctx.Variables()
			fmMust(t, e)
			if len(vars) != 1 || vars[0] == a {
				t.Fatal("fresh array not substituted", vars)
			}
		})
	}
}
