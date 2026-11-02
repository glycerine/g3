package duality

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDualityGetDefsOrientationOverwriteAndOpaqueContexts(t *testing.T) {
	ctx, m, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	zero := wrap(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
	one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	sort, err := m.GetSort(raw(zero))
	must(err)
	constant := func(text string) *Expr {
		name, err := m.Symbols.Intern(text)
		must(err)
		return wrap(m.MkConst(name, sort))
	}
	x, y, z := constant("defs-x"), constant("defs-y"), constant("defs-z")
	eq := func(a, b *Expr) *Expr { return wrap(m.MkEq(raw(a), raw(b))) }
	x0, x1, yx, z1 := eq(x, zero), eq(x, one), eq(y, x), eq(z, one)
	shared := wrap(m.MkAnd(raw(x0), raw(yx)))
	reversed := eq(one, z)
	bound := wrap(m.MkVar(0, sort))
	boundEquality := eq(bound, z)
	quantified := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(z1)}))
	disjoined := wrap(m.MkOr(raw(z1), raw(x1)))
	negated := wrap(m.MkNot(raw(z1)))
	le := wrap(m.MkArithmeticLE(raw(z), raw(one)))
	ge := wrap(m.MkArithmeticGE(raw(z), raw(one)))
	for _, tc := range []struct {
		name string
		args []*Expr
		want *Expr
	}{
		{"later-overwrites", []*Expr{shared, x1}, one},
		{"shared-revisit-overwrites", []*Expr{shared, x1, shared}, zero},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []ast.ASTHandle{}
			for _, e := range tc.args {
				args = append(args, raw(e))
			}
			for _, e := range []*Expr{reversed, boundEquality, quantified, disjoined, negated, le, ge} {
				args = append(args, raw(e))
			}
			input := wrap(m.MkAndArgs(args))
			defs, err := GetDefs(input)
			must(err)
			defer func() {
				for _, d := range defs {
					must(d.Close())
					must(d.Close())
				}
			}()
			if len(defs) != 2 {
				t.Fatalf("unexpected definition count: %d", len(defs))
			}
			if defs[raw(x)] == nil || raw(defs[raw(x)].Value) != raw(tc.want) {
				t.Fatal("definition overwrite order mismatch")
			}
			if defs[raw(y)] == nil || raw(defs[raw(y)].Value) != raw(x) {
				t.Fatal("definition was transitively substituted")
			}
			if raw(defs[raw(x)].Variable) != raw(x) {
				t.Fatal("map key owner mismatch")
			}
		})
	}
	// Each value owns references even after the only input wrapper is released.
	name, err := m.Symbols.Intern("defs-owned")
	must(err)
	h, err := m.MkConst(name, sort)
	must(err)
	equality, err := m.MkEq(h, raw(one))
	must(err)
	input, err := NewExpr(ctx, equality)
	must(err)
	defs, err := GetDefs(input)
	must(err)
	must(input.Close())
	owned := defs[h]
	if owned == nil {
		t.Fatal("missing owned definition")
	}
	if got, err := owned.Variable.Raw(); err != nil || got != h {
		t.Fatal("key lifetime lost", err)
	}
	if raw(owned.Value) != raw(one) {
		t.Fatal("value lifetime lost")
	}
	must(owned.Close())
}
