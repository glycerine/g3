package ackermannization_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

const ackTranslationScript = `(set-logic ALL)(declare-const x Bool)(declare-const y Bool)
(declare-fun f (Bool) Bool)(assert (not (= (f x) (f y))))`

func TestAckermannContextTranslationUsesConstructorSnapshot(t *testing.T) {
	source := ackContext(t, ackTranslationScript)
	sourceManager, err := source.Manager()
	ackMust(t, err)
	sourceServices := ackServices(t, source)
	p, err := sourceServices.Context.NewValue()
	ackMust(t, err)
	key := "max_steps"
	ackMust(t, sourceServices.Context.SetUIntName(&p, &key, 0))
	original, err := ackermannization.NewTactic(sourceManager, &p, sourceServices)
	ackMust(t, err)
	t.Cleanup(func() { ackMust(t, original.Close()) })
	ackMust(t, sourceServices.Context.SetUIntName(&p, &key, 10000))
	ackMust(t, original.UpdateParams(&p))
	ackMust(t, sourceServices.Context.Del(&p))
	target := ackContext(t, ackTranslationScript)
	targetManager, err := target.Manager()
	ackMust(t, err)
	targetServices := ackServices(t, target)
	if _, err = original.TranslateWithServices(targetManager, sourceServices); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("source services accepted for target", err)
	}
	translated, err := original.TranslateWithServices(targetManager, targetServices)
	ackMust(t, err)
	copy := translated.(*ackermannization.Tactic)
	t.Cleanup(func() { ackMust(t, copy.Close()) })
	ackMust(t, original.Close())
	ackMust(t, copy.UpdateParams(nil))
	goal, err := tactic.NewGoal(targetManager, false, true, false)
	ackMust(t, err)
	t.Cleanup(func() { ackMust(t, goal.Close()) })
	ackMust(t, target.AssertGoal(goal))
	var result tactic.TacticResult
	t.Cleanup(func() { ackMust(t, result.Close()) })
	if err = copy.Apply(goal, &result); err == nil || !strings.Contains(err.Error(), "max") {
		t.Fatal("translated tactic did not preserve constructor max_steps", err)
	}
}

func TestAckermannContextTranslationAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 128 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			source := ackContext(t, ackTranslationScript)
			sourceManager, err := source.Manager()
			ackMust(t, err)
			sourceServices := ackServices(t, source)
			p, err := sourceServices.Context.NewValue()
			ackMust(t, err)
			key := "max_steps"
			ackMust(t, sourceServices.Context.SetUIntName(&p, &key, 27))
			original, err := ackermannization.NewTactic(sourceManager, &p, sourceServices)
			ackMust(t, err)
			ackMust(t, sourceServices.Context.Del(&p))
			t.Cleanup(func() { ackMust(t, original.Close()) })
			target := ackContext(t, ackTranslationScript)
			targetManager, err := target.Manager()
			ackMust(t, err)
			targetServices := ackServices(t, target)
			targetManager.Budget().FailAfter(stop)
			translated, err := original.TranslateWithServices(targetManager, targetServices)
			targetManager.Budget().FailAfter(-1)
			injected := errors.Is(err, arena.ErrInjectedFailure)
			var failure memory.Failure
			memout := errors.As(err, &failure) && failure.Kind == memory.FailureMemout
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				ackMust(t, construction.Cleanup())
			}
			if err != nil {
				if !injected && !memout {
					t.Fatal("unexpected construction failure", err)
				}
				failures++
				return
			}
			success = true
			ackMust(t, translated.IncRef())
			ackMust(t, translated.DecRef())
		})
	}
	if !success || failures == 0 {
		t.Fatal("did not cover failure and success", failures, success)
	}
}
