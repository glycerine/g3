package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func apiParameterizedTacticGoal(t *testing.T, f *smtAPIFixture) *APIGoal {
	t.Helper()
	goal, err := f.c.NewGoal(true, false, false)
	astTestMust(t, err)
	boolean := f.pin(f.m.MkBoolSort())
	leftName, err := f.m.Symbols.Intern("apply-ex-left")
	astTestMust(t, err)
	rightName, err := f.m.Symbols.Intern("apply-ex-right")
	astTestMust(t, err)
	left := f.pin(f.m.MkConst(leftName, boolean))
	right := f.pin(f.m.MkConst(rightName, boolean))
	formula := f.pin(f.m.MkAnd(left, right))
	astTestMust(t, goal.Assert(formula))
	return goal
}

func apiSimplifyParameters(t *testing.T, f *smtAPIFixture) *ParameterSet {
	t.Helper()
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, parameters.SetBool("elim-and", true))
	return parameters
}

func apiParameterizedResultFormula(t *testing.T, result *APITacticResult) ast.ASTRef {
	t.Helper()
	subgoal, err := result.Subgoal(0)
	astTestMust(t, err)
	defer func() { astTestMust(t, subgoal.Close()) }()
	formula, err := subgoal.Formula(0)
	astTestMust(t, err)
	return formula
}
