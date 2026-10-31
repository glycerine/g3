package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArrayPluginRecognizesSafeStoreAndSelectChains(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	family, e := m.RegisterArrayPlugin()
	fmMust(t, e)
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a, b := f.constant("array-a", sort), f.constant("array-b", sort)
	i, j := f.constant("index-i", f.integer), f.constant("index-j", f.integer)
	one, two := f.number("1"), f.number("2")
	inner := f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, one}))
	outer := f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{inner, j, two}))
	selectA := f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, i}))
	selfIndex := f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, selectA, one}))
	ctx, e := NewSolverContext(m, []ast.ASTHandle{a})
	fmMust(t, e)
	p, e := NewArrayPlugin(ctx)
	fmMust(t, e)
	fmMust(t, ctx.AddPlugin(p))
	foundPlugin, e := ctx.PluginFor(a)
	fmMust(t, e)
	if foundPlugin != p {
		t.Fatal("context lookup")
	}
	idx, args, ok, e := p.arrayChain(outer, b, ast.ASTArrayOpStore)
	fmMust(t, e)
	if !ok || idx != 0 || len(args) != 2 || len(args[0]) != 2 || args[0][0] != j || args[0][1] != two || args[1][0] != i || args[1][1] != one {
		t.Fatalf("store chain %v %v %v", idx, args, ok)
	}
	idx, args, ok, e = p.arrayChain(selectA, one, ast.ASTArrayOpSelect)
	fmMust(t, e)
	if !ok || idx != 0 || len(args) != 1 || len(args[0]) != 1 || args[0][0] != i {
		t.Fatal("select", idx, args, ok)
	}
	for _, tc := range []struct {
		term, rhs ast.ASTHandle
		op        int32
	}{{a, b, ast.ASTArrayOpStore}, {selfIndex, b, ast.ASTArrayOpStore}, {outer, a, ast.ASTArrayOpStore}, {selectA, selectA, ast.ASTArrayOpSelect}, {outer, b, ast.ASTArrayOpSelect}} {
		_, _, ok, e = p.arrayChain(tc.term, tc.rhs, tc.op)
		fmMust(t, e)
		if ok {
			t.Fatal("unsafe chain accepted")
		}
	}
	fmMust(t, ctx.Close())
	_, _, _, e = p.arrayChain(outer, b, ast.ASTArrayOpStore)
	if !errors.Is(e, arena.ErrClosed) {
		t.Fatal("closed plugin", e)
	}
}

func TestArrayPluginEliminatesDirectEqualityInEitherOrientation(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "forward", true: "reverse"}[reverse], func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			family, e := m.RegisterArrayPlugin()
			fmMust(t, e)
			sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
			a, b := f.constant("a", sort), f.constant("b", sort)
			i := f.constant("i", f.integer)
			one := f.number("1")
			ctx, e := NewSolverContext(m, []ast.ASTHandle{a})
			fmMust(t, e)
			p, e := NewArrayPlugin(ctx)
			fmMust(t, e)
			fmMust(t, ctx.AddPlugin(p))
			defer func() { fmMust(t, ctx.Close()) }()
			self := f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, one}))
			unsafe := f.own(m.MkEq(a, self))
			solved, e := p.TryDirectEquality(unsafe, unsafe)
			fmMust(t, e)
			if solved {
				t.Fatal("self-referential equality eliminated")
			}
			var eq ast.ASTHandle
			if reverse {
				eq = f.own(m.MkEq(b, a))
			} else {
				eq = f.own(m.MkEq(a, b))
			}
			probe := f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, i}))
			guard := f.own(m.MkEq(probe, one))
			formula := f.own(m.MkAnd(eq, guard))
			solved, e = p.TryDirectEquality(eq, formula)
			fmMust(t, e)
			if !solved {
				t.Fatal("direct equality not solved")
			}
			count, e := ctx.NumVariables()
			fmMust(t, e)
			if count != 0 {
				t.Fatal("variable not eliminated", count)
			}
			actual, present, e := ctx.CurrentFormula()
			fmMust(t, e)
			if !present {
				t.Fatal("formula not published")
			}
			expectedEq := f.own(m.MkEq(b, b))
			selectB := f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{b, i}))
			expectedGuard := f.own(m.MkEq(selectB, one))
			expected := f.own(m.MkAnd(expectedEq, expectedGuard))
			if actual != expected {
				t.Fatal("substitution mismatch", actual, expected)
			}
		})
	}
}
