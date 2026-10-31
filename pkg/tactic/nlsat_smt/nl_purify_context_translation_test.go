package nlsatsmt

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
)

func TestNLPurifyContextTranslationOwnsTargetServices(t *testing.T) {
	sourceManager := astTestManager(t)
	_, _, source := nlModelEvaluatorFixture(t, sourceManager)
	p, err := source.Context.NewValue()
	astTestMust(t, err)
	key := "random_seed"
	astTestMust(t, source.Context.SetUIntName(&p, &key, 17))
	original, err := NewNLPurifyTactic(sourceManager, source.Context, source.Globals, &p, source)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, original.Close()) })
	astTestMust(t, source.Context.SetUIntName(&p, &key, 29))
	astTestMust(t, original.UpdateParams(&p))
	astTestMust(t, source.Context.SetUIntName(&p, &key, 31))
	astTestMust(t, source.Context.Del(&p))
	targetManager := astTestManager(t)
	_, _, target := nlModelEvaluatorFixture(t, targetManager)
	translated, err := original.TranslateWithServices(targetManager, target.Context, target.Globals, target)
	astTestMust(t, err)
	astTestMust(t, translated.IncRef())
	t.Cleanup(func() { astTestMust(t, translated.DecRef()) })
	copy, ok := translated.(*NLPurifyTactic)
	if !ok || copy.context != target.Context || copy.services.Cooperation != target.Cooperation || copy.smt == original.smt || copy.nonlinear == original.nonlinear {
		t.Fatal("translated tactic did not create independent target services")
	}
	seed, err := target.Context.GetUIntName(&copy.parameters, &key, 0)
	astTestMust(t, err)
	if seed != 29 {
		t.Fatal("translated seed", seed)
	}
	astTestMust(t, original.Close())
	truth, err := targetManager.MkTrue()
	astTestMust(t, err)
	goal, err := tactic.NewGoal(targetManager, false, true, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	astTestMust(t, goal.Assert(truth))
	var result tactic.TacticResult
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	astTestMust(t, copy.Apply(goal, &result))
	if !result.DecidedSat() {
		t.Fatal("translated nonlinear tactic did not solve target goal")
	}
}
