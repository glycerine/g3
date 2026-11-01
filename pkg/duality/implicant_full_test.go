package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"math/big"
	"testing"
)

func TestDualityFullImplicantAllBranchesAndSourceAtomOrder(t *testing.T) {
	ctx, m, s, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	must(g.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	p, err := ctx.BoolConst("full-implicant-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("full-implicant-q")
	must(err)
	closeExpr(t, q)
	ph, qh := raw(p), raw(q)
	nq := wrap(m.MkNot(qh))
	status, err := g.CheckUpdateModel(nil, []*Expr{p, nq})
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	disjunction := wrap(m.MkOr(ph, qh))
	implies := wrap(m.MkImplies(qh, ph))
	iff := wrap(m.MkIff(ph, qh))
	formula := wrap(m.MkAndArgs([]ast.ASTHandle{raw(disjunction), raw(nq), raw(implies), raw(iff), ph}))
	out, err := g.UnderapproxFullFormula(formula, true)
	must(err)
	defer out.Close()
	count, err := m.GetNumArgs(raw(out))
	must(err)
	if count != 2 {
		t.Fatal("shared atoms not deduplicated", count)
	}
	first, err := m.GetArg(raw(out), 0)
	must(err)
	second, err := m.GetArg(raw(out), 1)
	must(err)
	if first != ph || second != raw(nq) {
		t.Fatal("all-branch cube order/polarity changed")
	}
	// Full extraction includes q's false value, unlike satisfying-branch extraction.
	branch, err := g.UnderapproxFormula(disjunction, nil)
	must(err)
	defer branch.Close()
	if raw(branch) != ph {
		t.Fatal("satisfying-branch reference changed")
	}
	must(s.Push())
	must(s.Assert(raw(out)))
	must(s.Assert(qh))
	status, err = s.CheckSat(nil)
	must(err)
	must(s.Pop(1))
	if status != lbool.False {
		t.Fatal("full cube failed to constrain false atom", status)
	}
	numeral := wrap(m.ArithmeticNumeralRat(big.NewRat(7, 1), true))
	unknown, err := g.UnderapproxFullFormula(numeral, true)
	must(err)
	defer unknown.Close()
	if !m.IsTrue(raw(unknown)) {
		t.Fatal("unresolved non-Boolean value was included")
	}
}

func TestDualityFullImplicantArrayEqualityFiltering(t *testing.T) {
	ctx, m, s, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	must(g.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	zero := wrap(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
	one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	integer, err := m.GetSort(raw(zero))
	must(err)
	arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), integer)
	must(err)
	sortRef, err := m.NewRef(arraySort)
	must(err)
	defer func() { must(m.DelRef(&sortRef)) }()
	a := wrap(m.MkConstArray(arraySort, raw(zero)))
	b := wrap(m.MkConstArray(arraySort, raw(one)))
	equal := wrap(m.MkEq(raw(a), raw(a)))
	different := wrap(m.MkEq(raw(a), raw(b)))
	status, err := g.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	for _, extensional := range []bool{false, true} {
		accepted, err := g.UnderapproxFullFormula(equal, extensional)
		must(err)
		if raw(accepted) != raw(equal) {
			t.Fatal("identical model arrays omitted")
		}
		must(accepted.Close())
		omitted, err := g.UnderapproxFullFormula(different, extensional)
		must(err)
		if !m.IsTrue(raw(omitted)) {
			t.Fatal("false array equality included in full cube")
		}
		must(omitted.Close())
		negative := wrap(m.MkNot(raw(different)))
		omitted, err = g.UnderapproxFullFormula(negative, extensional)
		must(err)
		if !m.IsTrue(raw(omitted)) {
			t.Fatal("negation traversal bypassed array filtering")
		}
		must(omitted.Close())
	}
}
