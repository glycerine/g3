package nlsatsmt

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestNLPurifyLearnsInterfaceCoreAndPreservesContextDependencies(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	a := analysisConst(t, m, boolean, "core-a")
	b := analysisConst(t, m, boolean, "core-b")
	i := analysisConst(t, m, boolean, "core-interface-i")
	j := analysisConst(t, m, boolean, "core-interface-j")
	p := analysisConst(t, m, boolean, "core-formula-p")
	q := analysisConst(t, m, boolean, "core-formula-q")
	goal, err := tactic.NewGoal(m, false, true, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	for _, item := range [][2]ast.ASTHandle{{p, a}, {q, b}} {
		dep, err := m.DependencyLeaf(item[1])
		astTestMust(t, err)
		astTestMust(t, goal.AssertWithProof(item[0], ast.ASTHandle{}, dep))
	}
	extraction, err := solver.ExtractClausesAndDependencies(goal)
	astTestMust(t, err)
	defer func() { astTestMust(t, extraction.Close()) }()
	tracker, err := newNLCoreTracker(m, extraction)
	astTestMust(t, err)
	nonlinear, err := tactic.NewGoal(m, false, true, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, nonlinear.Close()) }()
	learned, err := tracker.learn([]ast.ASTHandle{a, i, b, j}, nonlinear)
	astTestMust(t, err)
	if !learned || len(tracker.used) != 2 || tracker.used[0] != a || tracker.used[1] != b {
		t.Fatal("core partition", learned, tracker.used)
	}
	notJ, err := m.MkNot(j)
	astTestMust(t, err)
	astTestPin(t, m, notJ)
	notI, err := m.MkNot(i)
	astTestMust(t, err)
	want, err := m.MkOr(notJ, notI)
	astTestMust(t, err)
	got, err := nonlinear.Form(0)
	astTestMust(t, err)
	if got != want {
		t.Fatal("blocking clause", got, want)
	}
	learned, err = tracker.learn([]ast.ASTHandle{a}, nonlinear)
	astTestMust(t, err)
	if learned || len(tracker.used) != 2 {
		t.Fatal("final context core", learned, tracker.used)
	}
	var core tactic.DependencyRef
	defer func() { astTestMust(t, core.Close()) }()
	astTestMust(t, tracker.finish(goal, &core))
	result, err := goal.Form(0)
	astTestMust(t, err)
	if !m.IsFalse(result) {
		t.Fatal("unsat goal", result)
	}
	leaves, err := m.DependencyLinearize(core.Get(), nil)
	astTestMust(t, err)
	if len(leaves) != 2 || !((leaves[0] == a && leaves[1] == b) || (leaves[0] == b && leaves[1] == a)) {
		t.Fatal("joined dependencies", leaves)
	}
}
