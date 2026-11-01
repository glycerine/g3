package qe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestQuantifierContextTranslationCopiesParametersAndOptions(t *testing.T) {
	source := newFMFixture(t)
	context := source.r.config.Context
	p, err := context.NewValue()
	fmMust(t, err)
	key := "qe_nonlinear"
	fmMust(t, context.SetBoolName(&p, &key, true))
	original, err := NewQuantifierTactic(source.r.manager, &p, source.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, original.Close()) })
	fmMust(t, context.Del(&p))
	target := newFMFixture(t)
	if _, err = original.TranslateWithServices(target.r.manager, source.r.config); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("source services accepted for target", err)
	}
	translated, err := original.TranslateWithServices(target.r.manager, target.r.config)
	fmMust(t, err)
	copy := translated.(*QuantifierTactic)
	t.Cleanup(func() { fmMust(t, copy.Close()) })
	if copy.manager != target.r.manager || copy.config.Context != target.r.config.Context || !copy.options.nonlinear {
		t.Fatal("translated tactic did not copy target services and options")
	}
	p, err = context.NewValue()
	fmMust(t, err)
	fmMust(t, context.SetBoolName(&p, &key, false))
	fmMust(t, original.UpdateParams(&p))
	fmMust(t, context.Del(&p))
	if !copy.options.nonlinear {
		t.Fatal("source update changed translated options")
	}
	fmMust(t, original.Close())
	g := target.goal(false, false)
	fmMust(t, g.Assert(target.quantifiedDefinition()))
	result := target.result()
	fmMust(t, copy.Apply(g, result))
	h, err := g.Form(0)
	fmMust(t, err)
	quantified, err := target.r.manager.HasQuantifiers(h)
	fmMust(t, err)
	if quantified {
		t.Fatal("translated tactic did not eliminate the quantifier")
	}
}

func TestQuantifierContextTranslationAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 256 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			source := newFMFixture(t)
			p, err := source.r.config.Context.NewValue()
			fmMust(t, err)
			key := "qe_nonlinear"
			fmMust(t, source.r.config.Context.SetBoolName(&p, &key, true))
			original, err := NewQuantifierTactic(source.r.manager, &p, source.r.config)
			fmMust(t, err)
			fmMust(t, source.r.config.Context.Del(&p))
			t.Cleanup(func() { fmMust(t, original.Close()) })
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
