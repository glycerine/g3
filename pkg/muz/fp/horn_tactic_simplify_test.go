package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
)

func TestHornSimplifyProducesTransformedGoalAndFiltersQuery(t *testing.T) {
	f := newHornLifecycleFixture(t, true)
	f.populate(t)
	result := &tactic.TacticResult{}
	t.Cleanup(func() { fpMust(t, result.Close()) })
	fpMust(t, f.tactic.Apply(f.goal, result))
	if len(result.Goals) != 1 || result.ModelConverter == nil {
		t.Fatal("simplify result", len(result.Goals), result.ModelConverter)
	}
	depth, err := result.Goals[0].Depth()
	fpMust(t, err)
	if depth != 1 {
		t.Fatal("simplify depth", depth)
	}
	size, err := result.Goals[0].Size()
	fpMust(t, err)
	if size == 0 {
		t.Fatal("simplify discarded all Horn rules")
	}
}

// Source: horn_tactic::simplify applies dl_transforms.cpp's default pipeline.
// P, P => Q, not Q is reduced by inlining to the synthetic query fact,
// whose replacement with false must produce a contradictory goal.
func TestHornSimplifyAppliesDefaultInlining(t *testing.T) {
	f := newHornLifecycleFixture(t, true)
	f.populate(t)
	result := &tactic.TacticResult{}
	t.Cleanup(func() { fpMust(t, result.Close()) })
	fpMust(t, f.tactic.Apply(f.goal, result))
	if len(result.Goals) != 1 {
		t.Fatal("simplify goals", len(result.Goals))
	}
	size, err := result.Goals[0].Size()
	fpMust(t, err)
	if size != 1 {
		t.Fatal("default Horn transformations did not reduce the derivation", size)
	}
	formula, err := result.Goals[0].Form(0)
	fpMust(t, err)
	if !f.manager.IsFalse(formula) {
		t.Fatal("inlined query fact did not become false")
	}
}
