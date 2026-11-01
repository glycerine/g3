package ufbv

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestMacroTacticContextTranslationCopiesKindAndPolicy(t *testing.T) {
	for _, kind := range []macroTacticKind{macroFinderKind, quasiMacrosKind} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			sourceManager, source := macroTacticFixture(t, false)
			params, err := source.Context.NewValue()
			cacheMust(t, err)
			key := "elim_and"
			cacheMust(t, source.Context.SetBoolName(&params, &key, true))
			original, err := newMacroTactic(sourceManager, source.Context, source.Globals, &params, source, kind)
			cacheMust(t, err)
			t.Cleanup(func() { cacheMust(t, original.Close()) })
			cacheMust(t, source.Context.Del(&params))
			targetManager, target := macroTacticFixture(t, false)
			if _, err = original.TranslateWithServices(targetManager, source.Context, source.Globals, source); !errors.Is(err, uparams.ErrParamsOwner) {
				t.Fatal("source services accepted for target", err)
			}
			translated, err := original.TranslateWithServices(targetManager, target.Context, target.Globals, target)
			cacheMust(t, err)
			copy := translated.(*MacroTactic)
			t.Cleanup(func() { cacheMust(t, copy.Close()) })
			if copy.kind != kind || copy.manager != targetManager || copy.context != target.Context || copy.globals != target.Globals || copy.services.Cooperation != target.Cooperation {
				t.Fatal("target tactic kind or services")
			}
			value, err := target.Context.GetBoolName(&copy.params, &key, false)
			cacheMust(t, err)
			if !value {
				t.Fatal("parameter snapshot")
			}
			cacheMust(t, original.UpdateParams(nil))
			cacheMust(t, original.Close())
			value, err = target.Context.GetBoolName(&copy.params, &key, false)
			cacheMust(t, err)
			if !value {
				t.Fatal("source mutation changed target policy")
			}
			quantifier, application, _ := macroTacticFormula(t, targetManager, kind == quasiMacrosKind)
			goal, err := tactic.NewGoal(targetManager, false, true, false)
			cacheMust(t, err)
			t.Cleanup(func() { cacheMust(t, goal.Close()) })
			cacheMust(t, goal.Assert(quantifier))
			cacheMust(t, goal.Assert(application))
			var result tactic.TacticResult
			t.Cleanup(func() { cacheMust(t, result.Close()) })
			cacheMust(t, copy.Apply(goal, &result))
			if result.ModelConverter == nil {
				t.Fatal("translated tactic omitted its model converter")
			}
		})
	}
}

func TestMacroTacticContextTranslationAllocationFrontiers(t *testing.T) {
	for _, kind := range []macroTacticKind{macroFinderKind, quasiMacrosKind} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			success, failures := false, 0
			for stop := int64(0); stop < 128 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					sourceManager, source := macroTacticFixture(t, false)
					params, err := source.Context.NewValue()
					cacheMust(t, err)
					key := "elim_and"
					cacheMust(t, source.Context.SetBoolName(&params, &key, true))
					original, err := newMacroTactic(sourceManager, source.Context, source.Globals, &params, source, kind)
					cacheMust(t, err)
					cacheMust(t, source.Context.Del(&params))
					t.Cleanup(func() { cacheMust(t, original.Close()) })
					targetManager, target := macroTacticFixture(t, false)
					targetManager.Budget().FailAfter(stop)
					translated, err := original.TranslateWithServices(targetManager, target.Context, target.Globals, target)
					targetManager.Budget().FailAfter(-1)
					injected := errors.Is(err, arena.ErrInjectedFailure)
					var failure memory.Failure
					memout := errors.As(err, &failure) && failure.Kind == memory.FailureMemout
					var construction memory.ConstructionFailure
					if errors.As(err, &construction) {
						cacheMust(t, construction.Cleanup())
					}
					if err != nil {
						if !injected && !memout {
							t.Fatal("unexpected translation failure", err)
						}
						failures++
						return
					}
					success = true
					cacheMust(t, translated.IncRef())
					cacheMust(t, translated.DecRef())
				})
			}
			if !success || failures == 0 {
				t.Fatal("did not cover failure and success", failures, success)
			}
		})
	}
}
