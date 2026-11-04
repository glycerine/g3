package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"math/big"
	"testing"
	"time"
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

func TestTimedSubtermTruthRestoresScopeOnError(t *testing.T) {
	ctx, _, _ := dualityNativeSolverFixture(t)
	g, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	profile := NewProfiler(nil)
	var tick int64
	profile.now = func() time.Time { tick++; return time.Unix(tick, 0) }
	ctx.activeProfiler = profile
	defer func() { ctx.activeProfiler = nil }()
	if err := profile.TimerStart("outer"); err != nil {
		t.Fatal(err)
	}
	outer := profile.current
	if _, err := g.timedSubtermTruth(make(map[ast.ASTHandle]int), ast.ASTHandle{}); err == nil {
		t.Fatal("invalid subterm accepted")
	}
	if profile.current != outer || outer.children["SubtermTruth"].elapsed != time.Second {
		t.Fatal("failed subterm evaluation did not close its timer")
	}
	profile.now = nil
	if _, err := g.timedSubtermTruth(make(map[ast.ASTHandle]int), ast.ASTHandle{}); err == nil {
		t.Fatal("timer start failure ignored")
	}
	if profile.current != outer {
		t.Fatal("failed timer start changed scope")
	}
}

func TestSubtermTruthAllocationFailureRetry(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(native))
	must(g.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	p, err := ctx.BoolConst("subterm-retry-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("subterm-retry-q")
	must(err)
	closeExpr(t, q)
	ph, err := p.Raw()
	must(err)
	qh, err := q.Raw()
	must(err)
	nq, err := m.MkNot(qh)
	must(err)
	notQ, err := NewExpr(ctx, nq)
	must(err)
	defer func() { must(notQ.Close()) }()
	formulaHandle, err := m.MkAnd(ph, nq)
	must(err)
	formula, err := NewExpr(ctx, formulaHandle)
	must(err)
	defer func() { must(formula.Close()) }()
	must(native.Assert(ph))
	must(native.Assert(nq))
	status, err := g.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal("model setup failed", status)
	}
	baseline := m.Stats().LiveReferences
	complete := false
	for allowance := int64(0); allowance < 128; allowance++ {
		m.Budget().FailAfter(allowance)
		value, failure := g.SubtermTruth(formula)
		m.Budget().FailAfter(-1)
		if failure == nil {
			if value != 1 {
				t.Fatal("unexpected truth value", value)
			}
			complete = true
		} else {
			retry, retryErr := g.SubtermTruth(formula)
			must(retryErr)
			if retry != 1 {
				t.Fatal("retry changed truth value", retry)
			}
		}
		if got := m.Stats().LiveReferences; got != baseline {
			t.Fatal("subterm truth leaked temporary references", allowance, got, baseline)
		}
		if complete {
			break
		}
	}
	if !complete {
		t.Fatal("allocation campaign did not reach successful truth evaluation")
	}
}
