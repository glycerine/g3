package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp Boolean-ITE guard and
// equal-branch BR_REWRITE1 path.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestBlastTermITELeavesBooleanChoiceAndCollapsesEqualTermBranches(t *testing.T) {
	f := newBlastTermITEFixture(t, false)
	_, thenBool, elseBool := propagateAtoms(t, f.m)
	booleanChoice, err := f.m.MkIte(f.cond, thenBool, elseBool)
	simplifyMust(t, err)
	booleanGoal := f.goal(t, booleanChoice, false, false)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, f.runner.Apply(booleanGoal, &result))
	got, err := booleanGoal.Form(0)
	simplifyMust(t, err)
	if got != booleanChoice || f.runner.numFresh != 0 {
		t.Fatal("Boolean ITE was blasted", got, f.runner.numFresh)
	}
	choice, err := f.m.MkIte(f.cond, f.one, f.one)
	simplifyMust(t, err)
	app, err := f.m.MkApp(f.f, []ast.ASTHandle{choice})
	simplifyMust(t, err)
	formula, err := f.m.MkEq(app, f.two)
	simplifyMust(t, err)
	termGoal := f.goal(t, formula, false, false)
	simplifyMust(t, f.runner.Apply(termGoal, &result))
	wantApp, err := f.m.MkApp(f.f, []ast.ASTHandle{f.one})
	simplifyMust(t, err)
	want, err := f.m.MkEq(wantApp, f.two)
	simplifyMust(t, err)
	got, err = termGoal.Form(0)
	simplifyMust(t, err)
	if got != want || f.runner.numFresh != 1 {
		t.Fatal("equal-branch collapse", got, want, f.runner.numFresh)
	}
}
