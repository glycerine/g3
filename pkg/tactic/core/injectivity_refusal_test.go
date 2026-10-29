package core

// Source: src/tactic/core/injectivity_tactic.cpp proof/core refusal order.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestInjectivityRefusesUnsatCoresBeforeProofs(t *testing.T) {
	f := newInjectivityFixture(t)
	coreGoal := f.goal(t, false, true, f.axiom)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	err := f.runner.Apply(coreGoal, &result)
	if err == nil || err.Error() != "injectivity does not support unsat core production" {
		t.Fatal("core refusal", err)
	}
	simplifyMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
	proofGoal := f.goal(t, true, false, f.axiom)
	err = f.runner.Apply(proofGoal, &result)
	if err == nil || err.Error() != "injectivity does not support proof production" {
		t.Fatal("proof refusal", err)
	}
}
