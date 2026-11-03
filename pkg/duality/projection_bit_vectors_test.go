package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source: duality_rpfp.cpp VariableProjector::GaussElimCheap/ProjectFormula.
// Generic equality projection must preserve modular and signed semantics;
// arithmetic bound elimination does not apply to bit-vector comparisons.
func TestProjectFormulaBitVectorDefinitionsAndModularConstraints(t *testing.T) {
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
	numeral := func(value uint32) *Expr { return wrap(m.BitVectorNumeralUnsigned(value, 4)) }
	zero, one, seven, eight, fifteen := numeral(0), numeral(1), numeral(7), numeral(8), numeral(15)
	sort, err := m.GetSort(raw(zero))
	must(err)
	constant := func(name string) *Expr { sym, e := m.Symbols.Intern(name); must(e); return wrap(m.MkConst(sym, sort)) }
	x, y, z := constant("projection-bv-x"), constant("projection-bv-y"), constant("projection-bv-z")
	eq := func(a, b *Expr) *Expr { return wrap(m.MkEq(raw(a), raw(b))) }
	add := func(a, b *Expr) *Expr { return wrap(m.MkBitVectorAdd(raw(a), raw(b))) }
	ule := func(a, b *Expr) *Expr { return wrap(m.MkBitVectorULE(raw(a), raw(b))) }
	sle := func(a, b *Expr) *Expr { return wrap(m.MkBitVectorSLE(raw(a), raw(b))) }
	conjunction := func(terms ...*Expr) *Expr {
		hs := make([]ast.ASTHandle, len(terms))
		for i, e := range terms {
			hs[i] = raw(e)
		}
		return wrap(m.MkAndArgs(hs))
	}
	disjunction := func(a, b *Expr) *Expr { return wrap(m.MkOr(raw(a), raw(b))) }
	definition := eq(x, add(y, one))
	for _, tc := range []struct {
		name          string
		keep          []*Expr
		formula, want *Expr
		eliminated    []*Expr
	}{
		{"unsigned-wrap", []*Expr{y}, conjunction(definition, ule(x, y)), eq(y, fifteen), []*Expr{x}},
		{"signed-wrap", []*Expr{y}, conjunction(definition, sle(x, y)), eq(y, seven), []*Expr{x}},
		{"noninvertible-coefficient", []*Expr{y}, conjunction(eq(x, add(y, y)), eq(x, zero)), disjunction(eq(y, zero), eq(y, eight)), []*Expr{x}},
		{"reversed-alias", []*Expr{y}, conjunction(eq(add(y, one), x), eq(x, z), ule(z, y)), eq(y, fifteen), []*Expr{x, z}},
		{"preserved-comparison", []*Expr{x, y}, ule(x, y), ule(x, y), nil},
		{"cyclic-increment", nil, eq(x, add(x, one)), wrap(m.MkFalse()), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mustSub := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			rawSub := func(e *Expr) ast.ASTHandle {
				t.Helper()
				h, err := e.Raw()
				mustSub(err)
				return h
			}
			wrapSub := func(h ast.ASTHandle, err error) *Expr {
				t.Helper()
				mustSub(err)
				e, err := NewExpr(ctx, h)
				mustSub(err)
				owned = append(owned, e)
				return e
			}
			projected, err := graph.ProjectFormula(tc.keep, tc.formula, simplifier)
			mustSub(err)
			defer func() { mustSub(projected.Close()) }()
			h := rawSub(projected)
			absent := make(map[ast.ASTHandle]bool)
			for _, e := range tc.eliminated {
				absent[rawSub(e)] = true
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
					t.Fatal("eliminated bit-vector survived projection")
				}
				kind, e := m.GetKind(n)
				mustSub(e)
				if kind == ast.ASTApp {
					arity, e := m.GetNumArgs(n)
					mustSub(e)
					for k := uint32(0); k < arity; k++ {
						child, e := m.GetArg(n, k)
						mustSub(e)
						todo = append(todo, child)
					}
				}
			}
			same := wrapSub(m.MkIff(h, rawSub(tc.want)))
			neg := wrapSub(m.MkNot(rawSub(same)))
			mustSub(native.Push())
			defer func() { mustSub(native.Pop(1)) }()
			mustSub(native.Assert(rawSub(neg)))
			status, err := native.CheckSat(nil)
			mustSub(err)
			if status != lbool.False {
				t.Fatal("bit-vector projection differs from independently derived modular formula", status)
			}
		})
	}
}
