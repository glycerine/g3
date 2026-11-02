package duality

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDualityIneqToEqAdjacencyShapeAndNativeEquivalence(t *testing.T) {
	ctx, m, native := dualityNativeSolverFixture(t)
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
	sort, err := m.GetSort(raw(zero))
	must(err)
	name, err := m.Symbols.Intern("ineq-x")
	must(err)
	x := wrap(m.MkConst(name, sort))
	le := wrap(m.MkArithmeticLE(raw(x), raw(zero)))
	ge := wrap(m.MkArithmeticGE(raw(x), raw(zero)))
	reverse := wrap(m.MkArithmeticGE(raw(zero), raw(x)))
	strict := wrap(m.MkArithmeticGT(raw(x), raw(zero)))
	eq := wrap(m.MkEq(raw(x), raw(zero)))
	yes := wrap(m.MkTrue())
	for _, tc := range []struct {
		name        string
		input, want []*Expr
	}{
		{"le-ge", []*Expr{le, ge}, []*Expr{eq, yes}},
		{"ge-le", []*Expr{ge, le}, []*Expr{eq, yes}},
		{"same-direction", []*Expr{le, le}, []*Expr{le, le}},
		{"reversed-operands", []*Expr{le, reverse}, []*Expr{le, reverse}},
		{"non-adjacent", []*Expr{le, yes, ge}, []*Expr{le, yes, ge}},
		{"strict", []*Expr{le, strict}, []*Expr{le, strict}},
		{"overlap", []*Expr{le, ge, le}, []*Expr{eq, yes, le}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := make([]ast.ASTHandle, len(tc.input))
			for i, e := range tc.input {
				args[i] = raw(e)
			}
			input := wrap(m.MkAndArgs(args))
			output, err := IneqToEq(input)
			must(err)
			defer output.Close()
			for i, want := range tc.want {
				got, err := m.GetArg(raw(output), uint32(i))
				must(err)
				if got != raw(want) {
					t.Fatalf("argument %d changed incorrectly", i)
				}
			}
			different := wrap(m.MkNot(raw(wrap(m.MkIff(raw(input), raw(output))))))
			must(native.Push())
			must(native.Assert(raw(different)))
			status, err := native.CheckSat(nil)
			must(err)
			must(native.Pop(1))
			if status != lbool.False {
				t.Fatalf("rewrite not equivalent: %v", status)
			}
		})
	}
	pair := wrap(m.MkAnd(raw(le), raw(ge)))
	pattern := wrap(m.MkPattern([]ast.ASTHandle{raw(pair)}))
	q := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(pair), Patterns: []ast.ASTHandle{raw(pattern)}, Weight: 11}))
	root := wrap(m.MkAnd(raw(pair), raw(q)))
	output, err := IneqToEq(root)
	must(err)
	defer output.Close()
	outside, err := m.GetArg(raw(output), 0)
	must(err)
	quant, err := m.GetArg(raw(output), 1)
	must(err)
	body, err := m.GetQuantifierBody(quant)
	must(err)
	if body != outside {
		t.Fatal("shared rewritten body not reused")
	}
	pat, err := m.GetPattern(quant, 0)
	must(err)
	if pat != raw(pattern) {
		t.Fatal("quantifier pattern changed")
	}
	weight, err := m.GetQuantifierWeight(quant)
	must(err)
	if weight != 11 {
		t.Fatal("quantifier metadata changed")
	}
	original, err := m.GetArg(raw(pair), 0)
	must(err)
	if original != raw(le) {
		t.Fatal("input mutated")
	}
}
