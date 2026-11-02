package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source: duality_rpfp.cpp VariableProjector::GaussElimCheap/ProjectFormula.
// Array-valued equality definitions use the same dependency-ordered projection
// as scalar constants before the arithmetic phase examines surviving literals.
func TestProjectFormulaArrayDefinitionsAndSelectConstraints(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer func() { must(simplifier.Close()) }()
	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	var owned []*Expr
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			must(owned[i].Close())
		}
	}()
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		owned = append(owned, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	zero := wrap(m.ArithmeticIntegerNumeralText("0"))
	one := wrap(m.ArithmeticIntegerNumeralText("1"))
	two := wrap(m.ArithmeticIntegerNumeralText("2"))
	integer, err := m.GetSort(raw(zero))
	must(err)
	arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), integer)
	must(err)
	constant := func(name string, sort ast.ASTHandle) *Expr {
		s, e := m.Symbols.Intern(name)
		must(e)
		return wrap(m.MkConst(s, sort))
	}
	a, b, c := constant("projection-array-a", arraySort), constant("projection-array-b", arraySort), constant("projection-array-c", arraySort)
	i, j := constant("projection-array-i", integer), constant("projection-array-j", integer)
	family, err := m.ArrayFamilyID()
	must(err)
	store := func(a, i, v *Expr) *Expr {
		return wrap(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{raw(a), raw(i), raw(v)}))
	}
	selectAt := func(a, i *Expr) *Expr {
		return wrap(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{raw(a), raw(i)}))
	}
	eq := func(a, b *Expr) *Expr { return wrap(m.MkEq(raw(a), raw(b))) }
	conjunction := func(terms ...*Expr) *Expr {
		hs := make([]ast.ASTHandle, len(terms))
		for i, e := range terms {
			hs[i] = raw(e)
		}
		return wrap(m.MkAndArgs(hs))
	}
	truth := wrap(m.MkTrue())
	ac := eq(a, store(c, i, one))
	ba := eq(b, a)
	for _, tc := range []struct {
		name          string
		keep          []*Expr
		formula, want *Expr
		eliminated    []*Expr
	}{
		{"store-select", []*Expr{c, i}, conjunction(ac, eq(selectAt(a, i), one)), truth, []*Expr{a}},
		{"contradictory-select", []*Expr{c, i}, conjunction(ac, eq(selectAt(a, i), two)), wrap(m.MkFalse()), []*Expr{a}},
		{"alias-store-chain", []*Expr{c, i, j}, conjunction(ba, ac, eq(selectAt(b, j), two)), eq(selectAt(store(c, i, one), j), two), []*Expr{a, b}},
		{"reversed-equalities", []*Expr{c, i, j}, conjunction(eq(store(c, i, one), a), eq(a, b), eq(selectAt(b, j), two)), eq(selectAt(store(c, i, one), j), two), []*Expr{a, b}},
		{"kept-array", []*Expr{a, c, i}, ac, ac, nil},
		{"cyclic-array", []*Expr{i}, eq(a, store(a, i, one)), eq(selectAt(a, i), one), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projected, err := graph.ProjectFormula(tc.keep, tc.formula, simplifier)
			must(err)
			defer func() { must(projected.Close()) }()
			h := raw(projected)
			absent := make(map[ast.ASTHandle]bool)
			for _, e := range tc.eliminated {
				absent[raw(e)] = true
			}
			todo := []ast.ASTHandle{h}
			seen := make(map[ast.ASTHandle]bool)
			for len(todo) > 0 {
				n := todo[len(todo)-1]
				todo = todo[:len(todo)-1]
				if seen[n] {
					continue
				}
				seen[n] = true
				if absent[n] {
					t.Fatal("eliminated array survived projection")
				}
				kind, e := m.GetKind(n)
				must(e)
				if kind == ast.ASTApp {
					arity, e := m.GetNumArgs(n)
					must(e)
					for k := uint32(0); k < arity; k++ {
						child, e := m.GetArg(n, k)
						must(e)
						todo = append(todo, child)
					}
				}
			}
			same := wrap(m.MkIff(h, raw(tc.want)))
			neg := wrap(m.MkNot(raw(same)))
			must(native.Push())
			must(native.Assert(raw(neg)))
			status, err := native.CheckSat(nil)
			must(err)
			must(native.Pop(1))
			if status != lbool.False {
				t.Fatal("array projection differs from independently derived formula", status)
			}
		})
	}
}
