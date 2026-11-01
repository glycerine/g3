package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestDualityQuantifierSimplificationDistributionAndIndexShifts(t *testing.T) {
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
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	p, err := ctx.BoolConst("quant-simplify-p")
	must(err)
	closeExpr(t, p)
	boolean, err := m.GetSort(raw(p))
	must(err)
	v0, v1 := wrap(m.MkVar(0, boolean)), wrap(m.MkVar(1, boolean))
	quant := func(forall bool, arity int, body *Expr) *Expr {
		sorts := make([]ast.ASTHandle, arity)
		names := make([]usymbol.Symbol, arity)
		for i := range sorts {
			sorts[i] = boolean
			names[i] = usymbol.Numerical(uint32(i))
		}
		return wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Body: raw(body), Weight: 11}))
	}
	// Expand Boolean binders independently for the native solver: direct
	// quantified equivalence may return unknown in its current quantifier engine.
	expand := func(q *Expr) *Expr {
		t.Helper()
		body, err := m.GetQuantifierBody(raw(q))
		must(err)
		var values []ast.ASTHandle
		for _, value := range []bool{false, true} {
			var constant *Expr
			if value {
				constant = wrap(m.MkTrue())
			} else {
				constant = wrap(m.MkFalse())
			}
			ref, err := substituteExpression(ctx, body, map[ast.ASTHandle]ast.ASTHandle{raw(v0): raw(constant)})
			must(err)
			h, err := m.RefGet(ref)
			must(err)
			owned := wrap(h, nil)
			must(m.DelRef(&ref))
			values = append(values, raw(owned))
		}
		universal, err := m.IsForall(raw(q))
		must(err)
		if universal {
			return wrap(m.MkAndArgs(values))
		}
		return wrap(m.MkOrArgs(values))
	}
	for _, forall := range []bool{false, true} {
		q := quant(forall, 1, v0)
		var body *Expr
		if forall {
			body = wrap(m.MkAnd(raw(v0), raw(p)))
		} else {
			body = wrap(m.MkOr(raw(v0), raw(p)))
		}
		distributed, err := ctx.CloneQuantAndSimp(q, body)
		must(err)
		defer distributed.Close()
		first, err := m.GetArg(raw(distributed), 0)
		must(err)
		if first != raw(q) {
			t.Fatal("dependent distribution branch changed")
		}
		second, err := m.GetArg(raw(distributed), 1)
		must(err)
		if second != raw(p) {
			t.Fatal("unused distribution binder not removed")
		}
		if forall {
			body = wrap(m.MkOr(raw(v0), raw(p)))
		} else {
			body = wrap(m.MkAnd(raw(v0), raw(p)))
		}
		partial, err := ctx.CloneQuantAndSimp(q, body)
		must(err)
		defer partial.Close()
		first, err = m.GetArg(raw(partial), 0)
		must(err)
		second, err = m.GetArg(raw(partial), 1)
		must(err)
		if first != raw(q) || second != raw(p) {
			t.Fatal("independent junct not pushed out")
		}
		original := quant(forall, 1, body)
		expandedOriginal := expand(original)
		expandedInner := expand(q)
		ref, err := substituteExpression(ctx, raw(partial), map[ast.ASTHandle]ast.ASTHandle{raw(q): raw(expandedInner)})
		must(err)
		expandedRaw, err := m.RefGet(ref)
		must(err)
		expandedPartial := wrap(expandedRaw, nil)
		must(m.DelRef(&ref))
		equal := wrap(m.MkIff(raw(expandedOriginal), raw(expandedPartial)))
		difference := wrap(m.MkNot(raw(equal)))
		must(s.Push())
		must(s.Assert(raw(difference)))
		status, err := s.CheckSat(nil)
		must(err)
		must(s.Pop(1))
		if status != lbool.False {
			t.Fatal("quantifier push changed semantics", status)
		}
		shifted, err := ctx.CloneQuantAndSimp(q, v1)
		must(err)
		defer shifted.Close()
		if raw(shifted) != raw(v0) {
			t.Fatal("dropping binder did not shift outer variable")
		}
		negative := wrap(m.MkNot(raw(v0)))
		dual, err := ctx.CloneQuantAndSimp(q, negative)
		must(err)
		defer dual.Close()
		isNot, inner, err := m.IsNot(raw(dual))
		must(err)
		if !isNot {
			t.Fatal("negation not preserved")
		}
		innerForall, err := m.IsForall(inner)
		must(err)
		if innerForall == forall {
			t.Fatal("negation did not dualize quantifier")
		}
		weight, err := m.GetQuantifierWeight(inner)
		must(err)
		if weight != 11 {
			t.Fatal("quantifier weight changed")
		}
		multiple := quant(forall, 2, p)
		unchanged, err := ctx.CloneQuantAndSimp(multiple, p)
		must(err)
		defer unchanged.Close()
		if raw(unchanged) != raw(multiple) {
			t.Fatal("source unused multi-binder behavior changed")
		}
	}
}

func TestDualityQuantifierSimplificationNestedAnnotationsAndDeMorgan(t *testing.T) {
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
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	yes := wrap(m.MkTrue())
	boolean, err := m.GetSort(raw(yes))
	must(err)
	v0, v1 := wrap(m.MkVar(0, boolean)), wrap(m.MkVar(1, boolean))
	spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(yes)}
	patternBody := wrap(m.MkOr(raw(v0), raw(v1)))
	pattern := wrap(m.MkPattern([]ast.ASTHandle{raw(patternBody)}))
	spec.Patterns = []ast.ASTHandle{raw(pattern)}
	inner := wrap(m.MkQuantifier(spec))
	spec.Patterns = nil
	spec.Body = raw(inner)
	outer := wrap(m.MkQuantifier(spec))
	result, err := ctx.CloneQuantAndSimp(outer, inner)
	must(err)
	defer result.Close()
	// has_free uses used_vars, so a variable occurring only in a nested trigger
	// still prevents dropping this outer binder (unlike IsClosedFormula).
	if raw(result) != raw(outer) {
		t.Fatal("nested annotation free-variable dependency ignored")
	}
	spec.Body = raw(v0)
	single := wrap(m.MkQuantifier(spec))
	// A De Morgan child inside the partial-push branch is collected in source
	// order and re-negated without introducing double negation.
	independent := wrap(m.MkNot(raw(v1)))
	dependent := wrap(m.MkNot(raw(v0)))
	pair := wrap(m.MkAnd(raw(dependent), raw(independent)))
	negative := wrap(m.MkNot(raw(pair)))
	body := wrap(m.MkOr(raw(negative), raw(v0)))
	reduced, err := ctx.CloneQuantAndSimp(single, body)
	must(err)
	defer reduced.Close()
	right, err := m.GetArg(raw(reduced), 1)
	must(err)
	if right != raw(v0) {
		t.Fatal("De Morgan independent term not shifted/collected")
	}
}
