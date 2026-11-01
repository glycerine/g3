package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"math/big"
	"testing"
)

func TestRPFPSubtermTruthNativeBooleanConnectives(t *testing.T) {
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
	p, err := ctx.BoolConst("subterm-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("subterm-q")
	must(err)
	closeExpr(t, q)
	ph, err := p.Raw()
	must(err)
	qh, err := q.Raw()
	must(err)
	nq, err := q.Not()
	must(err)
	closeExpr(t, nq)
	nqh, err := nq.Raw()
	must(err)
	must(s.Assert(ph))
	must(s.Assert(nqh))
	status, err := g.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	check := func(h ast.ASTHandle, err error, want int) {
		t.Helper()
		must(err)
		term, err := NewExpr(ctx, h)
		must(err)
		defer term.Close()
		got, err := g.SubtermTruth(term)
		must(err)
		if got != want {
			t.Fatalf("truth=%d want=%d", got, want)
		}
	}
	check(ph, nil, 1)
	check(qh, nil, 0)
	check(nqh, nil, 1)
	h, err := m.MkAnd(ph, qh)
	check(h, err, 0)
	h, err = m.MkOr(ph, qh)
	check(h, err, 1)
	h, err = m.MkImplies(ph, qh)
	check(h, err, 0)
	h, err = m.MkImplies(qh, ph)
	check(h, err, 1)
	h, err = m.MkIff(ph, qh)
	check(h, err, 0)
	// Non-Boolean model results retain the source unresolved value.
	h, err = m.ArithmeticNumeralRat(big.NewRat(3, 1), true)
	check(h, err, 2)
	labelName, err := m.Symbols.Intern("subterm-label")
	must(err)
	h, err = m.MkLabel(true, []usymbol.Symbol{labelName}, ph)
	check(h, err, 1)
}
