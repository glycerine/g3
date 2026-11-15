package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDualityBooleanCommonFactorsAndSourceMultiplicity(t *testing.T) {
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
		term, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, term)
		return term
	}
	p, err := ctx.BoolConst("factor-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("factor-q")
	must(err)
	closeExpr(t, q)
	r, err := ctx.BoolConst("factor-r")
	must(err)
	closeExpr(t, r)
	ph, err := p.Raw()
	must(err)
	qh, err := q.Raw()
	must(err)
	rh, err := r.Raw()
	must(err)
	yes := wrap(m.MkTrue())
	no := wrap(m.MkFalse())
	for _, and := range []bool{false, true} {
		inner := func(args []ast.ASTHandle) *Expr {
			if and {
				return wrap(m.MkOrArgs(args))
			}
			return wrap(m.MkAndArgs(args))
		}
		left := inner([]ast.ASTHandle{ph, qh})
		right := inner([]ast.ASTHandle{ph, rh})
		out, err := ctx.ReallySimplifyAndOr([]*Expr{left, right}, and)
		must(err)
		defer out.Close()
		raw, err := out.Raw()
		must(err)
		first, err := m.GetArg(raw, 0)
		must(err)
		if first != ph {
			t.Fatal("common factor not pulled out")
		}
		rest, err := m.GetArg(raw, 1)
		must(err)
		restOp, err := m.GetDeclKind(rest)
		must(err)
		want := ast.ASTOpOr
		if and {
			want = ast.ASTOpAnd
		}
		if restOp != want {
			t.Fatal("incorrect factored remainder operator")
		}
		// Independently refute a difference between the original and factored form.
		lh, err := left.Raw()
		must(err)
		rr, err := right.Raw()
		must(err)
		var original *Expr
		if and {
			original = wrap(m.MkAnd(lh, rr))
		} else {
			original = wrap(m.MkOr(lh, rr))
		}
		originalRaw, err := original.Raw()
		must(err)
		eq := wrap(m.MkIff(originalRaw, raw))
		eqRaw, err := eq.Raw()
		must(err)
		neg := wrap(m.MkNot(eqRaw))
		negRaw, err := neg.Raw()
		must(err)
		must(s.Push())
		must(s.Assert(negRaw))
		status, err := s.CheckSat(nil)
		must(err)
		must(s.Pop(1))
		if status != lbool.False {
			t.Fatal("factoring changed Boolean semantics", status)
		}
		allCommon := inner([]ast.ASTHandle{ph, ph})
		absorption, err := ctx.ReallySimplifyAndOr([]*Expr{left, allCommon}, and)
		must(err)
		defer absorption.Close()
		absorptionRaw, err := absorption.Raw()
		must(err)
		if absorptionRaw != ph {
			t.Fatal("empty remainder did not absorb other factors")
		}
		disjoint := inner([]ast.ASTHandle{rh, rh})
		unchanged, err := ctx.ReallySimplifyAndOr([]*Expr{left, disjoint}, and)
		must(err)
		defer unchanged.Close()
		unchangedRaw, err := unchanged.Raw()
		must(err)
		disjointRaw, err := disjoint.Raw()
		must(err)
		unchangedLast, err := m.GetArg(unchangedRaw, 1)
		must(err)
		if unchangedLast != disjointRaw {
			t.Fatal("disjoint juncts were factored")
		}
		duplicateLeft := inner([]ast.ASTHandle{ph, ph, qh})
		duplicateRight := inner([]ast.ASTHandle{ph, ph, rh})
		duplicates, err := ctx.ReallySimplifyAndOr([]*Expr{duplicateLeft, duplicateRight}, and)
		must(err)
		defer duplicates.Close()
		dh, err := duplicates.Raw()
		must(err)
		count, err := m.GetNumArgs(dh)
		must(err)
		if count != 3 {
			t.Fatal("source multiset intersection multiplicity lost", count)
		}
		for i := uint32(0); i < 2; i++ {
			factor, err := m.GetArg(dh, i)
			must(err)
			if factor != ph {
				t.Fatal("duplicate common factor lost")
			}
		}
		neutral, absorbing := no, yes
		if and {
			neutral, absorbing = yes, no
		}
		identity, err := ctx.ReallySimplifyAndOr([]*Expr{neutral, p}, and)
		must(err)
		defer identity.Close()
		ih, err := identity.Raw()
		must(err)
		if ih != ph {
			t.Fatal("neutral constant survived")
		}
		absorbed, err := ctx.ReallySimplifyAndOr([]*Expr{p, absorbing}, and)
		must(err)
		defer absorbed.Close()
		ah, err := absorbed.Raw()
		must(err)
		expected, err := absorbing.Raw()
		must(err)
		if ah != expected {
			t.Fatal("absorbing constant ignored")
		}
		empty, err := ctx.ReallySimplifyAndOr(nil, and)
		must(err)
		defer empty.Close()
		eh, err := empty.Raw()
		must(err)
		expected, err = neutral.Raw()
		must(err)
		if eh != expected {
			t.Fatal("empty junct identity mismatch")
		}
		// One non-junct operand prevents factoring; source does not flatten it.
		mixed, err := ctx.ReallySimplifyAndOr([]*Expr{p, left}, and)
		must(err)
		defer mixed.Close()
		mh, err := mixed.Raw()
		must(err)
		mf, err := m.GetArg(mh, 0)
		must(err)
		if mf != ph {
			t.Fatal("mixed operand order changed")
		}
	}
}

// AUDIT.md duality_01-F2: PullCommonFactors only splits basic and/or;
// arithmetic > (op 5, like basic and) is left alone.
func TestDualityPullCommonFactorsIgnoresArithmeticOp(t *testing.T) {
	ctx, m, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	intSort, err := m.GetSort(one)
	must(err)
	name, err := m.Symbols.Intern("factor-arith-a")
	must(err)
	a, err := m.MkConst(name, intSort)
	must(err)
	g1, err := m.MkArithmeticGT(a, one)
	must(err)
	g2, err := m.MkArithmeticGT(a, two)
	must(err)
	e1, err := NewExpr(ctx, g1)
	must(err)
	closeExpr(t, e1)
	e2, err := NewExpr(ctx, g2)
	must(err)
	closeExpr(t, e2)
	out, err := ctx.ReallySimplifyAndOr([]*Expr{e1, e2}, false)
	must(err)
	closeExpr(t, out)
	h, err := out.Raw()
	must(err)
	family, err := m.GetFamilyID(h)
	must(err)
	op, err := m.GetDeclKind(h)
	must(err)
	if family != m.BasicFamilyID() || op != ast.ASTOpOr {
		t.Fatal("want basic or", family, op)
	}
	if n, e := m.GetNumArgs(h); e != nil || n != 2 {
		t.Fatal("or arity", n, e)
	}
	for i, want := range []ast.ASTHandle{g1, g2} {
		if got, e := m.GetArg(h, uint32(i)); e != nil || got != want {
			t.Fatal("or argument", i, got, want, e)
		}
	}
}
