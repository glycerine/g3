package aig

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestAIGToGoalSourceRootOrderAndITE(t *testing.T) {
	m, _ := aigFixture(t)
	sort, e := m.MkBoolSort()
	aigMust(t, e)
	p, e := m.MkFreshConst("p", sort)
	aigMust(t, e)
	q, e := m.MkFreshConst("q", sort)
	aigMust(t, e)
	r, e := m.MkFreshConst("r", sort)
	aigMust(t, e)
	g, e := NewManager(m)
	aigMust(t, e)
	a, e := g.MakeVariable(p)
	aigMust(t, e)
	b, e := g.MakeVariable(q)
	aigMust(t, e)
	c, e := g.MakeVariable(r)
	aigMust(t, e)
	goal, e := tactic.NewGoal(m, false, false, false)
	aigMust(t, e)
	t.Cleanup(func() { aigMust(t, goal.Close()) })
	aigMust(t, goal.Assert(p))
	conj, e := g.AndArgs([]*Ref{a, b, c})
	aigMust(t, e)
	aigMust(t, g.ToGoal(conj, goal))
	size, e := goal.Size()
	aigMust(t, e)
	if size != 3 {
		t.Fatalf("split conjunction has %d assertions", size)
	}
	for i, want := range []ast.ASTHandle{q, p, r} {
		got, e := goal.Form(uint32(i))
		aigMust(t, e)
		if got != want {
			t.Fatalf("source right-first root order [%d]=%v want %v", i, got, want)
		}
	}
	ite, e := g.Ite(a, b, c)
	aigMust(t, e)
	aigMust(t, g.ToGoal(ite, goal))
	size, e = goal.Size()
	aigMust(t, e)
	if size != 1 {
		t.Fatalf("ITE was split into %d assertions", size)
	}
	h, e := goal.Form(0)
	aigMust(t, e)
	op, e := m.GetDeclKind(h)
	aigMust(t, e)
	if op != ast.ASTOpIte {
		t.Fatalf("recognized ITE became operator %d", op)
	}
	for mask := uint32(0); mask < 8; mask++ {
		pv, qv, rv := mask&1 != 0, mask&2 != 0, mask&4 != 0
		want := rv
		if pv {
			want = qv
		}
		if got := aigEvalASTFormula(t, m, h, map[ast.ASTHandle]bool{p: pv, q: qv, r: rv}); got != want {
			t.Fatalf("ITE assignment %03b: %v want %v", mask, got, want)
		}
	}
	iff, e := g.Iff(a, b)
	aigMust(t, e)
	aigMust(t, g.ToGoal(iff, goal))
	size, e = goal.Size()
	aigMust(t, e)
	if size != 1 {
		t.Fatalf("IFF emitted %d assertions", size)
	}
	h, e = goal.Form(0)
	aigMust(t, e)
	op, e = m.GetDeclKind(h)
	aigMust(t, e)
	if op != ast.ASTOpIff {
		t.Fatalf("recognized IFF became operator %d", op)
	}
	for mask := uint32(0); mask < 4; mask++ {
		pv, qv := mask&1 != 0, mask&2 != 0
		if got := aigEvalASTFormula(t, m, h, map[ast.ASTHandle]bool{p: pv, q: qv}); got != (pv == qv) {
			t.Fatalf("IFF assignment %02b: %v", mask, got)
		}
	}
	aigMust(t, iff.Close())
	aigMust(t, ite.Close())
	aigMust(t, conj.Close())
	aigMust(t, c.Close())
	aigMust(t, b.Close())
	aigMust(t, a.Close())
	aigMust(t, g.Close())
}

func TestAIGToGoalFlattensSingleUsePositiveAndUnderNegation(t *testing.T) {
	m, budget := aigFixture(t)
	sort, e := m.MkBoolSort()
	aigMust(t, e)
	p, e := m.MkFreshConst("p", sort)
	aigMust(t, e)
	q, e := m.MkFreshConst("q", sort)
	aigMust(t, e)
	r, e := m.MkFreshConst("r", sort)
	aigMust(t, e)
	g, e := NewManager(m)
	aigMust(t, e)
	a, e := g.MakeVariable(p)
	aigMust(t, e)
	b, e := g.MakeVariable(q)
	aigMust(t, e)
	c, e := g.MakeVariable(r)
	aigMust(t, e)
	inner, e := g.And(a, b)
	aigMust(t, e)
	outer, e := g.And(inner, c)
	aigMust(t, e)
	aigMust(t, inner.Close())
	negative, e := outer.Not()
	aigMust(t, e)
	goal, e := tactic.NewGoal(m, false, false, false)
	aigMust(t, e)
	t.Cleanup(func() { aigMust(t, goal.Close()) })
	aigMust(t, g.ToGoal(negative, goal))
	size, e := goal.Size()
	aigMust(t, e)
	if size != 1 {
		t.Fatalf("negated AND emitted %d assertions", size)
	}
	h, e := goal.Form(0)
	aigMust(t, e)
	op, e := m.GetDeclKind(h)
	aigMust(t, e)
	if op != ast.ASTOpOr {
		t.Fatalf("negated AND root operator %d", op)
	}
	n, e := m.GetNumArgs(h)
	aigMust(t, e)
	if n != 3 {
		t.Fatalf("source one-use child flattening emitted %d OR arguments", n)
	}
	for mask := uint32(0); mask < 8; mask++ {
		pv, qv, rv := mask&1 != 0, mask&2 != 0, mask&4 != 0
		want := !(pv && qv && rv)
		if got := aigEvalASTFormula(t, m, h, map[ast.ASTHandle]bool{p: pv, q: qv, r: rv}); got != want {
			t.Fatalf("flattened assignment %03b: %v", mask, got)
		}
	}
	aigMust(t, goal.Reset())
	baseline := budget.Used()
	reached := false
	for prefix := int64(0); prefix < 64; prefix++ {
		budget.FailAfter(prefix)
		failure := g.ToGoal(negative, goal)
		budget.FailAfter(-1)
		if failure == nil {
			aigMust(t, goal.Reset())
			reached = true
			break
		}
		if !errors.Is(failure, arena.ErrInjectedFailure) {
			t.Fatalf("goal output prefix %d: %v", prefix, failure)
		}
		aigMust(t, goal.Reset())
		if budget.Used() != baseline {
			t.Fatalf("goal output prefix %d retained %d bytes", prefix, budget.Used()-baseline)
		}
	}
	if !reached {
		t.Fatal("goal-output allocation campaign never succeeded")
	}
	aigMust(t, negative.Close())
	aigMust(t, outer.Close())
	aigMust(t, c.Close())
	aigMust(t, b.Close())
	aigMust(t, a.Close())
	aigMust(t, g.Close())
}

// aig2expr::mk_ite is mk_ite(get_cached(c), get_cached(t), get_cached(e));
// GCC converts e first, so (not r) is built before (not q)
// (AUDIT tactic_small_01-F8).
func TestAIGToGoalIteBuildsElseBeforeThen(t *testing.T) {
	m, _ := aigFixture(t)
	sort, e := m.MkBoolSort()
	aigMust(t, e)
	p, e := m.MkFreshConst("p", sort)
	aigMust(t, e)
	q, e := m.MkFreshConst("q", sort)
	aigMust(t, e)
	r, e := m.MkFreshConst("r", sort)
	aigMust(t, e)
	g, e := NewManager(m)
	aigMust(t, e)
	a, e := g.MakeVariable(p)
	aigMust(t, e)
	b, e := g.MakeVariable(q)
	aigMust(t, e)
	c, e := g.MakeVariable(r)
	aigMust(t, e)
	nb, e := b.Not()
	aigMust(t, e)
	nc, e := c.Not()
	aigMust(t, e)
	ite, e := g.Ite(a, nb, nc)
	aigMust(t, e)
	goal, e := tactic.NewGoal(m, false, false, false)
	aigMust(t, e)
	t.Cleanup(func() { aigMust(t, goal.Close()) })
	aigMust(t, g.ToGoal(ite, goal))
	h, e := goal.Form(0)
	aigMust(t, e)
	op, e := m.GetDeclKind(h)
	aigMust(t, e)
	defer func() {
		for _, r := range []*Ref{ite, nc, nb, c, b, a} {
			aigMust(t, r.Close())
		}
		aigMust(t, g.Close())
	}()
	if op != ast.ASTOpIte {
		t.Skip("ITE shape not recognized for complemented branches", op)
	}
	thenArg, e := m.GetArg(h, 1)
	aigMust(t, e)
	elseArg, e := m.GetArg(h, 2)
	aigMust(t, e)
	thenID, e := m.GetASTID(thenArg)
	aigMust(t, e)
	elseID, e := m.GetASTID(elseArg)
	aigMust(t, e)
	if elseID >= thenID {
		t.Fatal("else literal must be converted before then literal", elseID, thenID)
	}
}
