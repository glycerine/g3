package sls

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func slsTranslationMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSLSContextTranslationCopiesParametersAndEngine(t *testing.T) {
	source, sourceManager := slsQFBVFactoryFixture(t)
	p, err := source.Context.NewValue()
	slsTranslationMust(t, err)
	key := "random_seed"
	slsTranslationMust(t, source.Context.SetUIntName(&p, &key, 17))
	original, err := NewCoreTactic(sourceManager, source.Context, source.Globals, &p)
	slsTranslationMust(t, err)
	t.Cleanup(func() { slsTranslationMust(t, original.Close()) })
	slsTranslationMust(t, source.Context.Del(&p))
	target, targetManager := slsQFBVFactoryFixture(t)
	if _, err = original.TranslateWithServices(targetManager, source.Context, source.Globals); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("source services accepted for target", err)
	}
	translated, err := original.TranslateWithServices(targetManager, target.Context, target.Globals)
	slsTranslationMust(t, err)
	copy := translated.(*CoreTactic)
	t.Cleanup(func() { slsTranslationMust(t, copy.Close()) })
	if copy.manager != targetManager || copy.context != target.Context || copy.globals != target.Globals || copy.owner == original.owner || copy.owner.engine == original.owner.engine {
		t.Fatal("translation did not create an independent target engine")
	}
	value, err := target.Context.GetUIntName(&copy.params, &key, 0)
	slsTranslationMust(t, err)
	if value != 17 {
		t.Fatal("translated seed", value)
	}
	p, err = source.Context.NewValue()
	slsTranslationMust(t, err)
	slsTranslationMust(t, source.Context.SetUIntName(&p, &key, 29))
	slsTranslationMust(t, original.UpdateParams(&p))
	slsTranslationMust(t, source.Context.Del(&p))
	value, err = target.Context.GetUIntName(&copy.params, &key, 0)
	slsTranslationMust(t, err)
	if value != 17 {
		t.Fatal("source update changed target seed", value)
	}
	slsTranslationMust(t, original.Close())
	atom := slsBoolConst(t, targetManager, "translated-sls-atom")
	slsPin(t, targetManager, atom)
	goal, err := tactic.NewGoal(targetManager, false, true, false)
	slsTranslationMust(t, err)
	t.Cleanup(func() { slsTranslationMust(t, goal.Close()) })
	slsTranslationMust(t, goal.Assert(atom))
	var result tactic.TacticResult
	t.Cleanup(func() { slsTranslationMust(t, result.Close()) })
	slsTranslationMust(t, copy.Apply(goal, &result))
	if len(result.Goals) != 1 || result.ModelConverter == nil {
		t.Fatal("translated engine did not publish a model")
	}
	if copy.CollectStatistics()["sls full evals"] == 0 {
		t.Fatal("translated engine did not evaluate the target goal")
	}
}

func TestSLSContextTranslationAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 256 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			source, sourceManager := slsQFBVFactoryFixture(t)
			p, err := source.Context.NewValue()
			slsTranslationMust(t, err)
			key := "random_seed"
			slsTranslationMust(t, source.Context.SetUIntName(&p, &key, 17))
			original, err := NewCoreTactic(sourceManager, source.Context, source.Globals, &p)
			slsTranslationMust(t, err)
			slsTranslationMust(t, source.Context.Del(&p))
			t.Cleanup(func() { slsTranslationMust(t, original.Close()) })
			target, targetManager := slsQFBVFactoryFixture(t)
			targetManager.Budget().FailAfter(stop)
			translated, err := original.TranslateWithServices(targetManager, target.Context, target.Globals)
			targetManager.Budget().FailAfter(-1)
			injected := errors.Is(err, arena.ErrInjectedFailure)
			var failure memory.Failure
			memout := errors.As(err, &failure) && failure.Kind == memory.FailureMemout
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				slsTranslationMust(t, construction.Cleanup())
			}
			if err != nil {
				if !injected && !memout {
					t.Fatal("unexpected construction failure", err)
				}
				failures++
				return
			}
			success = true
			slsTranslationMust(t, translated.IncRef())
			slsTranslationMust(t, translated.DecRef())
		})
	}
	if !success || failures == 0 {
		t.Fatal("did not cover failure and success", failures, success)
	}
}
