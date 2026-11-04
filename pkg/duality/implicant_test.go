package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"math/big"
	"testing"
	"time"
)

func TestRPFPImplicantSourceOrderPolarityAndITE(t *testing.T) {
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
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return h
	}
	p, err := ctx.BoolConst("implicant-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("implicant-q")
	must(err)
	closeExpr(t, q)
	ph, err := p.Raw()
	must(err)
	qh, err := q.Raw()
	must(err)
	must(s.Assert(ph))
	must(s.Assert(qh))
	status, err := g.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	extract := func(h ast.ASTHandle, ignore []*Expr) *Expr {
		t.Helper()
		formula, err := NewExpr(ctx, h)
		must(err)
		defer formula.Close()
		out, err := g.UnderapproxFormula(formula, ignore)
		must(err)
		closeExpr(t, out)
		return out
	}
	same := func(out *Expr, want ast.ASTHandle) {
		t.Helper()
		got, err := out.Raw()
		must(err)
		if got != want {
			t.Fatal("unexpected source literal selection", got, want)
		}
	}
	profile := NewProfiler(nil)
	var tick int64
	profile.now = func() time.Time { tick++; return time.Unix(tick, 0) }
	ctx.activeProfiler = profile
	same(extract(pin(m.MkOr(ph, qh)), nil), ph)
	child := profile.top.children["SubtermTruth"]
	if child == nil || child.elapsed != time.Second || profile.current != profile.top {
		t.Fatal("disjunction should time one selected subterm")
	}
	ctx.activeProfiler = nil
	same(extract(pin(m.MkImplies(ph, qh)), nil), qh)
	same(extract(pin(m.MkIff(ph, qh)), nil), qh)
	duplicate := pin(m.MkAnd(ph, ph))
	same(extract(duplicate, nil), ph)
	same(extract(pin(m.MkAnd(ph, qh)), []*Expr{p}), qh)
	yes := pin(m.MkTrue())
	same(extract(ph, []*Expr{p}), yes)
	np := pin(m.MkNot(ph))
	nq := pin(m.MkNot(qh))
	negAnd := pin(m.MkNot(pin(m.MkAnd(np, nq))))
	same(extract(negAnd, nil), ph)
	// Source emits no literal when no disjunct matches the current model.
	same(extract(pin(m.MkOr(np, nq)), nil), yes)
	one := pin(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	two := pin(m.ArithmeticNumeralRat(big.NewRat(2, 1), true))
	ite := pin(m.MkIte(ph, one, two))
	atom := pin(m.MkEq(ite, one))
	profile = NewProfiler(nil)
	tick = 0
	profile.now = func() time.Time { tick++; return time.Unix(tick, 0) }
	ctx.activeProfiler = profile
	out := extract(atom, nil)
	child = profile.top.children["SubtermTruth"]
	if child == nil || child.elapsed != time.Second || profile.current != profile.top {
		t.Fatal("ITE should time one condition")
	}
	ctx.activeProfiler = nil
	raw, err := out.Raw()
	must(err)
	n, err := m.GetNumArgs(raw)
	must(err)
	if n != 2 {
		t.Fatal("missing ITE selection literal", n)
	}
	first, err := m.GetArg(raw, 0)
	must(err)
	if first != ph {
		t.Fatal("ITE condition emitted out of source order")
	}
	// The selected arithmetic branch replaces the ITE in the resulting atom.
	second, err := m.GetArg(raw, 1)
	must(err)
	expected := pin(m.MkEq(one, one))
	if second != expected {
		t.Fatal("ITE branch not substituted")
	}
	ignored := extract(atom, []*Expr{p})
	same(ignored, expected)
	falseITE := pin(m.MkIte(np, one, two))
	falseAtom := pin(m.MkEq(falseITE, two))
	falseResult := extract(falseAtom, nil)
	falseRaw, err := falseResult.Raw()
	must(err)
	falseCondition, err := m.GetArg(falseRaw, 0)
	must(err)
	falseBranch, err := m.GetArg(falseRaw, 1)
	must(err)
	if falseCondition != ph || falseBranch != pin(m.MkEq(two, two)) {
		t.Fatal("false ITE branch/polarity mismatch")
	}

	// Check implication independently of the extraction's structural assertions.
	must(s.Push())
	must(s.Assert(raw))
	opposite := pin(m.MkNot(atom))
	must(s.Assert(opposite))
	status, err = s.CheckSat(nil)
	must(err)
	if status != lbool.False {
		t.Fatal("extracted ITE decisions do not imply input", status)
	}
	must(s.Pop(1))
}

func TestRPFPFixCurrentStateScopes(t *testing.T) {
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
	p, err := ctx.BoolConst("decision-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("decision-q")
	must(err)
	closeExpr(t, q)
	for _, term := range []*Expr{p, q} {
		decl, err := term.Decl()
		must(err)
		must(g.DeclareConstant(decl))
		must(decl.Close())
	}
	disjunction, err := p.Or(q)
	must(err)
	closeExpr(t, disjunction)
	f, err := NewTransformer(ctx, nil, nil, disjunction)
	must(err)
	defer func() { must(f.Close()) }()
	relation, err := ctx.BoolConst("decision-R")
	must(err)
	closeExpr(t, relation)
	node, err := g.CreateNode(relation)
	must(err)
	must(node.Bound.SetEmpty())
	edge, err := g.CreateEdge(node, f, nil)
	must(err)
	must(g.Push())
	must(g.AssertNode(node))
	must(g.AssertEdge(edge, 0, false, false))
	status, err := g.CheckUpdateModel(node, []*Expr{p, q})
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	must(g.Push())
	must(g.FixCurrentState(edge))
	if len(edge.constraints) != 1 {
		t.Fatal("decision not recorded on edge")
	}
	selected, err := m.RefGet(edge.constraints[0])
	must(err)
	ph, err := p.Raw()
	must(err)
	if selected != ph {
		t.Fatal("consequent-first decision not selected")
	}
	np, err := p.Not()
	must(err)
	closeExpr(t, np)
	nph, err := np.Raw()
	must(err)
	qh, err := q.Raw()
	must(err)
	status, err = s.CheckSat([]ast.ASTHandle{nph, qh})
	must(err)
	if status != lbool.False {
		t.Fatal("fixed decision allowed opposite model", status)
	}
	must(g.Pop(1))
	if len(edge.constraints) != 0 {
		t.Fatal("decision survived scope pop")
	}
	status, err = s.CheckSat([]ast.ASTHandle{nph, qh})
	must(err)
	if status != lbool.True {
		t.Fatal("popping decision failed to restore alternate model", status)
	}
	must(g.Pop(1))
	must(g.Push())
	must(g.FixCurrentState(edge))
	if len(edge.constraints) != 1 {
		t.Fatal("missing open-edge decision")
	}
	openDecision, err := m.RefGet(edge.constraints[0])
	must(err)
	if !m.IsTrue(openDecision) {
		t.Fatal("null edge formula did not select true")
	}
	must(g.Pop(1))
}
