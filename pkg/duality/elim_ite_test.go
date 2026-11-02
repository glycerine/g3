package duality

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDualityElimIteGuardedEqualitySharedNamesAndNativeSemantics(t *testing.T) {
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
	p, err := ctx.BoolConst("ite-condition")
	must(err)
	closeExpr(t, p)
	zero := wrap(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
	one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	conditional := wrap(m.MkIte(raw(p), raw(one), raw(zero)))
	direct := wrap(m.MkEq(raw(zero), raw(conditional)))
	yes := wrap(m.MkAnd(raw(p), raw(wrap(m.MkEq(raw(zero), raw(one))))))
	no := wrap(m.MkAnd(raw(wrap(m.MkNot(raw(p)))), raw(wrap(m.MkEq(raw(zero), raw(zero))))))
	expected := wrap(m.MkOr(raw(yes), raw(no)))
	rewritten, err := ElimIte(direct)
	must(err)
	defer rewritten.Close()
	if raw(rewritten) != raw(expected) {
		t.Fatal("right-hand conditional equality shape changed")
	}
	prove := func(formula *Expr) {
		t.Helper()
		must(native.Push())
		must(native.Assert(raw(formula)))
		status, err := native.CheckSat(nil)
		must(err)
		must(native.Pop(1))
		if status != lbool.False {
			t.Fatalf("expected UNSAT: %v", status)
		}
	}
	prove(wrap(m.MkNot(raw(wrap(m.MkIff(raw(direct), raw(rewritten)))))))
	gt := wrap(m.MkArithmeticGT(raw(conditional), raw(zero)))
	le := wrap(m.MkArithmeticLE(raw(conditional), raw(one)))
	root := wrap(m.MkAnd(raw(gt), raw(le)))
	output, err := ElimIte(root)
	must(err)
	defer output.Close()
	count, err := m.GetNumArgs(raw(output))
	must(err)
	if count != 2 {
		t.Fatal("shared conditional generated more than one definition")
	}
	main, err := m.GetArg(raw(output), 1)
	must(err)
	left, err := m.GetArg(main, 0)
	must(err)
	right, err := m.GetArg(main, 1)
	must(err)
	fresh, err := m.GetArg(left, 0)
	must(err)
	other, err := m.GetArg(right, 0)
	must(err)
	if fresh != other || fresh == raw(conditional) {
		t.Fatal("shared conditional name mismatch")
	}
	prove(wrap(m.MkAnd(raw(output), raw(wrap(m.MkNot(raw(root)))))))
	// Substitution of the original conditional witnesses extension of every
	// satisfying original model to the introduced constant.
	ref, err := substituteExpression(ctx, raw(output), map[ast.ASTHandle]ast.ASTHandle{fresh: raw(conditional)})
	must(err)
	defer m.DelRef(&ref)
	witness, err := m.RefGet(ref)
	must(err)
	prove(wrap(m.MkNot(raw(wrap(m.MkIff(raw(root), witness))))))
	// Quantifier bodies are rewritten but their patterns stay unchanged.
	sort, err := m.GetSort(raw(zero))
	must(err)
	pattern := wrap(m.MkPattern([]ast.ASTHandle{raw(conditional)}))
	q := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(direct), Patterns: []ast.ASTHandle{raw(pattern)}, Weight: 4}))
	quant, err := ElimIte(q)
	must(err)
	defer quant.Close()
	body, err := m.GetQuantifierBody(raw(quant))
	must(err)
	if body != raw(expected) {
		t.Fatal("quantifier body not rewritten")
	}
	pat, err := m.GetPattern(raw(quant), 0)
	must(err)
	if pat != raw(pattern) {
		t.Fatal("pattern changed")
	}
	weight, err := m.GetQuantifierWeight(raw(quant))
	must(err)
	if weight != 4 {
		t.Fatal("metadata changed")
	}
}
