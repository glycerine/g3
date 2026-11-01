package ufbv

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestUFBVRewriterAdapterContextTranslationUsesTargetState(t *testing.T) {
	sourceManager, source := macroTacticFixture(t, false)
	original, err := NewRewriterTacticAdapter(sourceManager, source.Context, source.Globals)
	cacheMust(t, err)
	t.Cleanup(func() { cacheMust(t, original.Close()) })
	tags := diagnostic.NewTraceTags()
	cacheMust(t, original.child.BindTrace(tags))
	targetManager, target := macroTacticFixture(t, false)
	if _, err = original.TranslateWithServices(targetManager, source.Context, source.Globals); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("source services accepted for target", err)
	}
	translated, err := original.TranslateWithServices(targetManager, target.Context, target.Globals)
	cacheMust(t, err)
	copy := translated.(*RewriterTacticAdapter)
	t.Cleanup(func() { cacheMust(t, copy.Close()) })
	if copy.context != target.Context || copy.globals != target.Globals || copy.child.manager != targetManager || copy.child.imp == original.child.imp || copy.child.trace != tags || copy.child.allocationLine != 85 {
		t.Fatal("target rewriter state")
	}
	cacheMust(t, original.Close())
	quantifier, application, _ := macroTacticFormula(t, targetManager, false)
	goal, err := tactic.NewGoal(targetManager, false, false, false)
	cacheMust(t, err)
	t.Cleanup(func() { cacheMust(t, goal.Close()) })
	cacheMust(t, goal.Assert(quantifier))
	cacheMust(t, goal.Assert(application))
	var result tactic.TacticResult
	t.Cleanup(func() { cacheMust(t, result.Close()) })
	cacheMust(t, copy.Apply(goal, &result))
	if len(result.Goals) != 1 || result.Goals[0] != goal {
		t.Fatal("translated rewriter did not publish the target goal")
	}
	if sorted, err := goal.IsWellSorted(); err != nil || !sorted {
		t.Fatal("translated result", sorted, err)
	}
}

func TestUFBVRewriterAdapterTranslationAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 128 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			sourceManager, source := macroTacticFixture(t, false)
			original, err := NewRewriterTacticAdapter(sourceManager, source.Context, source.Globals)
			cacheMust(t, err)
			t.Cleanup(func() { cacheMust(t, original.Close()) })
			targetManager, target := macroTacticFixture(t, false)
			targetManager.Budget().FailAfter(stop)
			translated, err := original.TranslateWithServices(targetManager, target.Context, target.Globals)
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
					t.Fatal("unexpected construction failure", err)
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
}

func TestUFBVRewriterAdapterConstructionAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 128 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			manager, services := macroTacticFixture(t, false)
			manager.Budget().FailAfter(stop)
			created, err := NewRewriterTacticAdapter(manager, services.Context, services.Globals)
			manager.Budget().FailAfter(-1)
			injected := errors.Is(err, arena.ErrInjectedFailure)
			var failure memory.Failure
			memout := errors.As(err, &failure) && failure.Kind == memory.FailureMemout
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				cacheMust(t, construction.Cleanup())
			}
			if err != nil {
				if !injected && !memout {
					t.Fatal("unexpected construction failure", err)
				}
				failures++
				return
			}
			success = true
			cacheMust(t, created.Close())
		})
	}
	if !success || failures == 0 {
		t.Fatal("did not cover failure and success", failures, success)
	}
}
