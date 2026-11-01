package qe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestLiteContextTranslationCopiesParametersAndRewriter(t *testing.T) {
	source := newFMFixture(t)
	context := source.r.config.Context
	p, err := context.NewValue()
	fmMust(t, err)
	key := "ignore_patterns_on_ground_qbody"
	fmMust(t, context.SetBoolName(&p, &key, false))
	original, err := NewLiteTactic(source.r.manager, &p, source.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, original.Close()) })
	fmMust(t, context.Del(&p))
	target := newFMFixture(t)
	if _, err = original.TranslateWithServices(target.r.manager, source.r.config); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("source services accepted for target", err)
	}
	translated, err := original.TranslateWithServices(target.r.manager, target.r.config)
	fmMust(t, err)
	copy := translated.(*LiteTactic)
	t.Cleanup(func() { fmMust(t, copy.Close()) })
	if copy.manager != target.r.manager || copy.implementation == original.implementation || copy.config.Context != target.r.config.Context {
		t.Fatal("translation did not create independent target state")
	}
	value, err := target.r.config.Context.GetBoolName(&copy.params, &key, true)
	fmMust(t, err)
	if value {
		t.Fatal("parameter was not copied")
	}
	fmMust(t, original.Close())
	g := target.goal(false, false)
	fmMust(t, g.Assert(target.quantifiedDefinition()))
	r := target.result()
	fmMust(t, copy.Apply(g, r))
	h, err := g.Form(0)
	fmMust(t, err)
	if h != target.predicate("tactic-p", target.number("3")) {
		t.Fatal("translated tactic did not eliminate the quantified variable")
	}
}

func TestLiteContextTranslationAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 512 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			source := newFMFixture(t)
			original := source.liteTactic()
			target := newFMFixture(t)
			target.r.manager.Budget().FailAfter(stop)
			translated, err := original.TranslateWithServices(target.r.manager, target.r.config)
			target.r.manager.Budget().FailAfter(-1)
			injected := errors.Is(err, arena.ErrInjectedFailure)
			var failure memory.Failure
			memout := errors.As(err, &failure) && failure.Kind == memory.FailureMemout
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				fmMust(t, construction.Cleanup())
			}
			if err != nil {
				if !injected && !memout {
					t.Fatal("unexpected construction failure", err)
				}
				failures++
				return
			}
			success = true
			fmMust(t, translated.IncRef())
			fmMust(t, translated.DecRef())
		})
	}
	if !success || failures == 0 {
		t.Fatal("did not cover failure and success", failures, success)
	}
}
