package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp proof and dependency update.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestBlastTermITEProofAndCoreDependency(t *testing.T) {
	f := newBlastTermITEFixture(t, true)
	choice, err := f.m.MkIte(f.cond, f.one, f.two)
	simplifyMust(t, err)
	app, err := f.m.MkApp(f.f, []ast.ASTHandle{choice})
	simplifyMust(t, err)
	formula, err := f.m.MkEq(app, f.one)
	simplifyMust(t, err)
	simplifyPin(t, f.m, formula)
	goal := f.goal(t, formula, true, true)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, f.runner.Apply(goal, &result))
	proof, err := goal.Proof(0)
	simplifyMust(t, err)
	rewritten, err := goal.Form(0)
	simplifyMust(t, err)
	fact, err := f.m.GetProofFact(proof)
	simplifyMust(t, err)
	if fact != rewritten {
		t.Fatal("modus ponens did not conclude rewritten formula")
	}
	dependency, err := goal.Dependency(0)
	simplifyMust(t, err)
	leaves, err := f.m.DependencyLinearize(dependency, nil)
	simplifyMust(t, err)
	if len(leaves) != 1 || leaves[0] != formula {
		t.Fatal("core dependency changed", leaves)
	}
}
