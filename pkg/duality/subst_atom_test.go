package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestDualitySubstAtomSourceSkeletonAndNegation(t *testing.T) {
	ctx, m, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	p, err := ctx.BoolConst("subst-atom-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("subst-atom-q")
	must(err)
	closeExpr(t, q)
	r, err := ctx.BoolConst("subst-atom-r")
	must(err)
	closeExpr(t, r)
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	ph, qh, rh := raw(p), raw(q), raw(r)
	yes, no := wrap(m.MkTrue()), wrap(m.MkFalse())
	check := func(formula, atom, value *Expr, want ast.ASTHandle) {
		t.Helper()
		out, err := ctx.SubstAtom(formula, atom, value)
		must(err)
		defer func() { must(out.Close()) }()
		if raw(out) != want {
			t.Fatal("source atom substitution mismatch", raw(out), want)
		}
	}
	np := wrap(m.MkNot(ph))
	check(p, p, yes, raw(yes))
	check(np, p, yes, raw(no))
	check(np, p, no, raw(yes))
	nq := wrap(m.MkNot(qh))
	check(np, p, nq, qh)
	check(np, p, q, raw(nq))
	disj := wrap(m.MkOr(ph, qh))
	check(disj, p, no, qh)
	conj := wrap(m.MkAnd(ph, qh))
	check(conj, p, no, raw(no))
	nested := wrap(m.MkAnd(raw(disj), rh))
	check(nested, p, no, raw(wrap(m.MkAnd(qh, rh))))
	// Source recurses into the skeleton before testing exact identity: even
	// replacing a conjunction by itself does not replace that whole node.
	check(conj, conj, no, raw(conj))
	implication := wrap(m.MkImplies(ph, qh))
	check(implication, p, no, raw(implication))
	equivalence := wrap(m.MkIff(ph, qh))
	check(equivalence, p, no, raw(equivalence))
	negJunct := wrap(m.MkNot(raw(disj)))
	check(negJunct, p, no, raw(negJunct))
	// Repeated input DAG nodes share memoized output while owning references.
	shared := wrap(m.MkOr(raw(nested), raw(nested)))
	out, err := ctx.SubstAtom(shared, p, no)
	must(err)
	defer out.Close()
	count, err := m.GetNumArgs(raw(out))
	must(err)
	if count != 2 {
		t.Fatal("source repeated-factor result changed", count)
	}
	// Under the selected literal's value, the substituted formula is equivalent.
	reduced, err := ctx.SubstAtom(nested, p, no)
	must(err)
	defer reduced.Close()
	equivalent := wrap(m.MkIff(raw(nested), raw(reduced)))
	difference := wrap(m.MkNot(raw(equivalent)))
	must(s.Push())
	must(s.Assert(raw(np)))
	must(s.Assert(raw(difference)))
	status, err := s.CheckSat(nil)
	must(err)
	must(s.Pop(1))
	if status != lbool.False {
		t.Fatal("substitution changed formula under its literal assignment", status)
	}

}

func TestDualitySubstAtomQuantifiedAtomTraversal(t *testing.T) {
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
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	p, err := ctx.BoolConst("subst-quant-p")
	must(err)
	closeExpr(t, p)
	ph, err := p.Raw()
	must(err)
	boolean, err := m.GetSort(ph)
	must(err)
	yes := wrap(m.MkTrue())
	no := wrap(m.MkFalse())
	inner := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: ph}))
	ih, err := inner.Raw()
	must(err)
	direct, err := ctx.SubstAtom(inner, inner, no)
	must(err)
	defer direct.Close()
	dh, err := direct.Raw()
	must(err)
	if !m.IsFalse(dh) {
		t.Fatal("quantified atom not replaced")
	}
	unchanged, err := ctx.SubstAtom(inner, p, no)
	must(err)
	defer unchanged.Close()
	uh, err := unchanged.Raw()
	must(err)
	if uh != ih {
		t.Fatal("ordinary atom substituted under quantifier")
	}
	body := wrap(m.MkAnd(ih, ph))
	bh, err := body.Raw()
	must(err)
	outer := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(1)}, Body: bh, Weight: 7}))
	reduced, err := ctx.SubstAtom(outer, inner, yes)
	must(err)
	defer reduced.Close()
	rh, err := reduced.Raw()
	must(err)
	rb, err := m.GetQuantifierBody(rh)
	must(err)
	if rb != ph {
		t.Fatal("quantified atom did not substitute in enclosing body")
	}
	forall, err := m.IsForall(rh)
	must(err)
	if forall {
		t.Fatal("quantifier kind changed")
	}
	weight, err := m.GetQuantifierWeight(rh)
	must(err)
	if weight != 7 {
		t.Fatal("quantifier weight changed", weight)
	}

}
