package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestDualityGroundLiteralsQuantifierContextsAndOpaqueNegation(t *testing.T) {
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
	p, err := ctx.BoolConst("ground-case-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("ground-case-q")
	must(err)
	closeExpr(t, q)
	r, err := ctx.BoolConst("ground-case-r")
	must(err)
	closeExpr(t, r)
	ph, qh, rh := raw(p), raw(q), raw(r)
	boolean, err := m.GetSort(ph)
	must(err)
	variable := wrap(m.MkVar(0, boolean))
	nr := wrap(m.MkNot(rh))
	nv := wrap(m.MkNot(raw(variable)))
	nested := wrap(m.MkOr(qh, ph))
	implies := wrap(m.MkImplies(qh, rh))
	iff := wrap(m.MkIff(ph, qh))
	opaque := wrap(m.MkNot(raw(wrap(m.MkAnd(ph, rh)))))
	body := wrap(m.MkAndArgs([]ast.ASTHandle{ph, raw(nested), raw(nr), raw(variable), raw(nv), raw(implies), raw(iff), raw(opaque)}))
	quant := func(body *Expr, patterns []ast.ASTHandle) *Expr {
		return wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(body), Patterns: patterns}))
	}
	quantified := quant(body, nil)
	root := wrap(m.MkAndArgs([]ast.ASTHandle{ph, raw(quantified), raw(quantified)}))
	check := func(formula *Expr, want []ast.ASTHandle) {
		t.Helper()
		lits, err := GetGroundLitsUnderQuants(formula)
		must(err)
		defer func() {
			for _, lit := range lits {
				must(lit.Close())
			}
		}()
		if len(lits) != len(want) {
			t.Fatal("candidate count", len(lits), len(want))
		}
		for i, lit := range lits {
			if raw(lit) != want[i] {
				t.Fatal("candidate order/polarity mismatch", i)
			}
		}
	}
	check(root, []ast.ASTHandle{ph, qh, raw(nr), rh, raw(opaque)})
	check(body, nil)
	negatedQuant := wrap(m.MkNot(raw(quantified)))
	check(negatedQuant, nil)
	nestedQuant := quant(quantified, nil)
	check(nestedQuant, []ast.ASTHandle{ph, qh, raw(nr), rh, raw(opaque)})
	// Patterns are not candidate sources, even when they contain ground terms.
	triggerBody := wrap(m.MkAnd(raw(variable), rh))
	trigger := wrap(m.MkPattern([]ast.ASTHandle{raw(triggerBody)}))
	annotated := quant(p, []ast.ASTHandle{raw(trigger)})
	check(annotated, []ast.ASTHandle{ph})
	yes := wrap(m.MkTrue())
	check(quant(yes, nil), []ast.ASTHandle{raw(yes)})
	// Caller-owned candidates survive the temporary input wrapper's release.
	temporary, err := quantified.Clone()
	must(err)
	candidates, err := GetGroundLitsUnderQuants(temporary)
	must(err)
	must(temporary.Close())
	for _, candidate := range candidates {
		if _, err := candidate.Raw(); err != nil {
			t.Fatal(err)
		}
		must(candidate.Close())
	}
}
