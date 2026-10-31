package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestRecover01GoalTransformation(t *testing.T) {
	base, m := fixDLFixture(t, false)
	sort, err := m.MkBoolSort()
	equationMust(t, err)
	symbol, err := m.Symbols.Intern("apply-p")
	equationMust(t, err)
	p, err := m.MkConst(symbol, sort)
	equationMust(t, err)
	fixDLPin(t, m, p)
	notP, err := m.MkNot(p)
	equationMust(t, err)
	fixDLPin(t, m, notP)
	x := fixDLConst(t, m, true, "apply-x")
	clause := func(lit ast.ASTHandle, value string) ast.ASTHandle {
		eq := collectorEq(t, m, x, fixDLNumber(t, m, true, value))
		c, e := m.MkOrArgs([]ast.ASTHandle{lit, eq})
		equationMust(t, e)
		fixDLPin(t, m, c)
		return c
	}
	c0, c1 := clause(p, "0"), clause(notP, "3")
	goal := fixDLGoal(t, m, false, true, false, c0, c1, p)
	var out tactic.TacticResult
	defer func() { equationMust(t, out.Close()) }()
	equationMust(t, recover01Apply(goal, base.rewriter, base.services, 10, &out))
	if len(out.Goals) != 1 || out.Goals[0] == goal || out.ModelConverter == nil {
		t.Fatal("missing transformed goal or model converter")
	}
	size, err := out.Goals[0].Size()
	equationMust(t, err)
	if size != 3 {
		t.Fatalf("got %d formulas, want two bounds and Boolean substitution", size)
	}
	originalSize, err := goal.Size()
	equationMust(t, err)
	if originalSize != 3 {
		t.Fatal("original goal changed")
	}
}

func TestRecover01IncompleteTableLeavesOriginalGoal(t *testing.T) {
	base, m := fixDLFixture(t, false)
	sort, err := m.MkBoolSort()
	equationMust(t, err)
	symbol, err := m.Symbols.Intern("missing-p")
	equationMust(t, err)
	p, err := m.MkConst(symbol, sort)
	equationMust(t, err)
	fixDLPin(t, m, p)
	x := fixDLConst(t, m, true, "missing-x")
	eq := collectorEq(t, m, x, fixDLNumber(t, m, true, "0"))
	clause, err := m.MkOrArgs([]ast.ASTHandle{p, eq})
	equationMust(t, err)
	fixDLPin(t, m, clause)
	goal := fixDLGoal(t, m, false, true, false, clause)
	var out tactic.TacticResult
	defer func() { equationMust(t, out.Close()) }()
	equationMust(t, recover01Apply(goal, base.rewriter, base.services, 10, &out))
	if len(out.Goals) != 1 || out.Goals[0] != goal || out.ModelConverter != nil {
		t.Fatal("incomplete table should leave goal unchanged")
	}
}

func TestRecover01RejectsProofAndCoreGoals(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		base, m := fixDLFixture(t, proofs)
		goal := fixDLGoal(t, m, proofs, false, !proofs)
		var out tactic.TacticResult
		if err := recover01Apply(goal, base.rewriter, base.services, 10, &out); err == nil {
			t.Fatal("unsupported goal accepted")
		}
		equationMust(t, out.Close())
	}
}
