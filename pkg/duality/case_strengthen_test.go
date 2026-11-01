package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"math/big"
	"testing"
)

func TestDualityCaseStrengtheningModelPolarityAndPairedLiterals(t *testing.T) {
	ctx, m, s, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	config := derivationEvaluatorFixture(t, ctx, globals)
	engine, err := qe.NewLite(m, nil, true, qe.Config{Context: ctx.config.params, Globals: globals, Cooperation: config.Cooperation})
	must(err)
	defer func() { must(engine.Close()) }()
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer func() { must(simplifier.Close()) }()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	must(g.BindModelEvaluator(config))
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	p, err := ctx.BoolConst("case-model-p")
	must(err)
	closeExpr(t, p)
	ph := raw(p)
	np := wrap(m.MkNot(ph))
	boolean, err := m.GetSort(ph)
	must(err)
	variable := wrap(m.MkVar(0, boolean))
	quantified := func(body *Expr) *Expr {
		return wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(body)}))
	}
	positive := quantified(wrap(m.MkAnd(raw(variable), ph)))
	negative := quantified(wrap(m.MkAnd(raw(variable), raw(np))))
	// Failure after collecting a case must release its independent output copy.
	if out, cases, err := g.StrengthenFormulaByCaseSplitting(positive, simplifier, engine); err == nil || out != nil || cases != nil {
		t.Fatal("missing-model error/output ownership", err)
	}
	for _, truth := range []bool{false, true} {
		assumption := np
		if truth {
			assumption = p
		}
		status, err := g.CheckUpdateModel(nil, []*Expr{assumption})
		must(err)
		if status != lbool.True {
			t.Fatal(status)
		}
		for _, neg := range []bool{false, true} {
			input, lit := positive, p
			if neg {
				input, lit = negative, np
			}
			out, cases, err := g.StrengthenFormulaByCaseSplitting(input, simplifier, engine)
			must(err)
			if len(cases) != 1 || raw(cases[0]) != raw(lit) {
				t.Fatal("source case literal selection/polarity changed")
			}
			// qe-lite rewrites quantifiers, leaving ordinary surrounding
			// applications in source form; normalize only the test observation.
			observedRef, err := simplifier.Simplify(raw(out))
			must(err)
			observed, err := m.RefGet(observedRef)
			must(err)
			if truth != neg {
				if observed != raw(lit) {
					t.Fatal("model fact not preserved in strengthened formula")
				}
			} else if !m.IsFalse(observed) {
				t.Fatal("false model case not reduced")
			}
			must(m.DelRef(&observedRef))
			// Here exists b. (b and literal) is exactly literal; check strengthening
			// against that independent Boolean interpretation using the native solver.
			difference := wrap(m.MkAnd(raw(out), raw(wrap(m.MkNot(raw(lit))))))
			must(s.Push())
			must(s.Assert(raw(difference)))
			status, err = s.CheckSat(nil)
			must(err)
			must(s.Pop(1))
			if status != lbool.False {
				t.Fatal("strengthened case does not imply input", status)
			}
			must(out.Close())
			for _, lit := range cases {
				must(lit.Close())
			}
		}
	}
	paired := quantified(wrap(m.MkOrArgs([]ast.ASTHandle{raw(variable), ph, raw(np)})))
	out, cases, err := g.StrengthenFormulaByCaseSplitting(paired, simplifier, engine)
	must(err)
	if len(cases) != 0 || !m.IsTrue(raw(out)) {
		t.Fatal("paired literals were split or tautology changed")
	}
	must(out.Close())
}

func TestDualityAdjustQuantifiersNativeQELite(t *testing.T) {
	ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	services := derivationEvaluatorFixture(t, ctx, globals)
	engine, err := qe.NewLite(m, nil, true, qe.Config{Context: ctx.config.params, Globals: globals, Cooperation: services.Cooperation})
	must(err)
	defer func() { must(engine.Close()) }()
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	three := wrap(m.ArithmeticNumeralRat(big.NewRat(3, 1), true))
	integer, err := m.GetSort(raw(three))
	must(err)
	variable := wrap(m.MkVar(0, integer))
	body := wrap(m.MkEq(raw(variable), raw(three)))
	formula := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(body)}))
	adjusted, err := AdjustQuantifiers(formula, engine)
	must(err)
	defer adjusted.Close()
	if !m.IsTrue(raw(adjusted)) {
		t.Fatal("native qe-lite did not eliminate existential definition")
	}
	originalKind, err := m.GetKind(raw(formula))
	must(err)
	if originalKind != ast.ASTQuantifier {
		t.Fatal("input mutated")
	}
	negated := wrap(m.MkNot(raw(formula)))
	changed, err := AdjustQuantifiers(negated, engine)
	must(err)
	defer changed.Close()
	isNot, child, err := m.IsNot(raw(changed))
	must(err)
	if !isNot || !m.IsTrue(child) {
		t.Fatal("qe-lite did not preserve source not-true application after eliminating its quantifier")
	}
	if _, err := AdjustQuantifiers(formula, nil); err == nil {
		t.Fatal("quantified adjustment accepted missing engine")
	}
	copied, err := AdjustQuantifiers(three, nil)
	must(err)
	defer copied.Close()
	if raw(copied) != raw(three) {
		t.Fatal("quantifier-free shortcut changed input")
	}
}
