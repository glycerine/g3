package sls

// Whole-source behavioral audit for src/tactic/sls/sls_tracker.h.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSLSTrackerWholeSourceWeightsSATCandidatesAndForgetting(t *testing.T) {
	_, search, evaluator, tracker, manager, numbers := slsEngineFixture(t)
	local := tracker.context.EmptyValue()
	forget, paws := "walksat_ucb_forget", "paws_init"
	if err := tracker.context.SetDoubleName(&local, &forget, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := tracker.context.SetUIntName(&local, &paws, 3); err != nil {
		t.Fatal(err)
	}
	if err := tracker.UpdateParams(&local); err != nil {
		t.Fatal(err)
	}
	p := slsBoolConst(t, manager, "sls-whole-p")
	q := slsBoolConst(t, manager, "sls-whole-q")
	root, err := manager.MkAnd(p, q)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, p, q, root)
	assertions := []ast.ASTHandle{root}
	if err = search.Initialize(assertions); err != nil {
		t.Fatal(err)
	}
	if tracker.weights[root] != 3 {
		t.Fatal("source paws_init", tracker.weights[root])
	}
	if err = search.IncreaseWeight(root); err != nil || tracker.weights[root] != 4 {
		t.Fatal("increase weight", tracker.weights[root], err)
	}
	if err = search.DecreaseWeight(root); err != nil || tracker.weights[root] != 3 {
		t.Fatal("decrease weight", tracker.weights[root], err)
	}
	if err = search.DecreaseWeight(root); err != nil || tracker.weights[root] != 3 {
		t.Fatal("paws floor", tracker.weights[root], err)
	}
	if err = search.Reset(); err != nil {
		t.Fatal(err)
	}
	if sat, err := tracker.IsSAT(); err != nil || sat {
		t.Fatal("all-zero conjunction", sat, err)
	}
	candidates, err := search.UnsatConstants(assertions)
	if err != nil || len(candidates) != 2 || candidates[0] != managerDeclHandle(t, manager, p) || candidates[1] != managerDeclHandle(t, manager, q) {
		t.Fatal("source insertion-ordered unsatisfied candidates", candidates, err)
	}
	one := numeral.NewInt(1)
	for _, candidate := range candidates {
		if err = evaluator.SeriousUpdate(search, candidate, &one); err != nil {
			t.Fatal(err)
		}
	}
	if sat, err := tracker.IsSAT(); err != nil || !sat {
		t.Fatal("satisfied conjunction", sat, err)
	}

	score := tracker.scores[root]
	tracker.touched += 8
	score.touched = 9
	if err = tracker.ucbForgetExpressions(assertions); err != nil {
		t.Fatal(err)
	}
	if score.touched != 5 {
		t.Fatal("source UCB affine forgetting", score.touched)
	}
	value, err := tracker.GetValue(root)
	if err != nil {
		t.Fatal(err)
	}
	if oneValue, err := numbers.IsOne(value); err != nil || !oneValue {
		t.Fatal("published root value", oneValue, err)
	}
}

func managerDeclHandle(t *testing.T, manager *ast.ASTManager, node ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	decl, err := manager.GetDecl(node)
	if err != nil {
		t.Fatal(err)
	}
	return decl
}
