package qe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestSATContextTranslationModesAndParameters(t *testing.T) {
	for _, name := range []string{"qe-sat", "qsat", "qe2", "qe_rec", "nlqsat", "nlqe"} {
		t.Run(name, func(t *testing.T) {
			source, target := newFMFixture(t), newFMFixture(t)
			fmMust(t, source.r.manager.RegisterDeclarationPlugins())
			fmMust(t, target.r.manager.RegisterDeclarationPlugins())
			p, err := source.r.config.Context.NewValue()
			fmMust(t, err)
			key := "projection_mode"
			fmMust(t, source.r.config.Context.SetBoolName(&p, &key, false))
			seed := "seed"
			fmMust(t, source.r.config.Context.SetUIntName(&p, &seed, 17))
			var original tactic.Tactic
			switch name {
			case "qe-sat":
				original, err = NewQESATTactic(source.r.manager, &p, source.r.config)
			case "qsat":
				original, err = NewQSATTactic(source.r.manager, &p, source.r.config)
			case "qe2":
				original, err = NewQE2Tactic(source.r.manager, &p, source.r.config)
			case "qe_rec":
				original, err = NewQERecursiveTactic(source.r.manager, &p, source.r.config)
			case "nlqsat":
				original, err = NewNLQSATTacticWithParams(source.r.manager, &p, source.r.config)
			case "nlqe":
				original, err = NewNLQETacticWithParams(source.r.manager, &p, source.r.config)
			}
			fmMust(t, err)
			fmMust(t, source.r.config.Context.Del(&p))
			defer func() { fmMust(t, original.(interface{ Close() error }).Close()) }()
			var copy tactic.Tactic
			switch item := original.(type) {
			case *QESATTactic:
				if _, err = item.TranslateWithServices(target.r.manager, source.r.config); !errors.Is(err, uparams.ErrParamsOwner) {
					t.Fatal("foreign services", err)
				}
				copy, err = item.TranslateWithServices(target.r.manager, target.r.config)
			case *QSATTactic:
				if _, err = item.TranslateWithServices(target.r.manager, source.r.config); !errors.Is(err, uparams.ErrParamsOwner) {
					t.Fatal("foreign services", err)
				}
				copy, err = item.TranslateWithServices(target.r.manager, target.r.config)
			case *NLQSATTactic:
				if _, err = item.TranslateWithServices(target.r.manager, source.r.config); !errors.Is(err, uparams.ErrParamsOwner) {
					t.Fatal("foreign services", err)
				}
				copy, err = item.TranslateWithServices(target.r.manager, target.r.config)
			}
			fmMust(t, err)
			defer func() { fmMust(t, copy.(interface{ Close() error }).Close()) }()
			switch item := original.(type) {
			case *QESATTactic:
				if item.options.projection || !copy.(*QESATTactic).options.projection {
					t.Fatal("qe-sat translation did not restore source defaults")
				}
			case *QSATTactic:
				targetTactic := copy.(*QSATTactic)
				if targetTactic.mode != item.mode || targetTactic.search == item.search || targetTactic.manager != target.r.manager {
					t.Fatal("QSAT mode or target ownership")
				}
			case *NLQSATTactic:
				targetTactic := copy.(*NLQSATTactic)
				value, e := target.r.config.Context.GetUIntName(&targetTactic.params, &seed, 0)
				fmMust(t, e)
				if targetTactic.mode != item.mode || targetTactic.manager != target.r.manager || value != 17 {
					t.Fatal("nonlinear mode or parameter snapshot", value)
				}
				fmMust(t, item.UpdateParams(nil))
				value, e = target.r.config.Context.GetUIntName(&targetTactic.params, &seed, 0)
				fmMust(t, e)
				if value != 17 {
					t.Fatal("source update changed target parameters", value)
				}
			}
		})
	}
}

func TestSATContextTranslationAllocationFrontiers(t *testing.T) {
	for _, name := range []string{"qe-sat", "qsat", "nlqsat"} {
		t.Run(name, func(t *testing.T) {
			success, failures := false, 0
			for stop := int64(0); stop < 1024 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					source, target := newFMFixture(t), newFMFixture(t)
					fmMust(t, source.r.manager.RegisterDeclarationPlugins())
					fmMust(t, target.r.manager.RegisterDeclarationPlugins())
					var original tactic.Tactic
					var err error
					switch name {
					case "qe-sat":
						original, err = NewQESATTactic(source.r.manager, nil, source.r.config)
					case "qsat":
						original, err = NewQSATTactic(source.r.manager, nil, source.r.config)
					case "nlqsat":
						p, e := source.r.config.Context.NewValue()
						fmMust(t, e)
						seed := "seed"
						fmMust(t, source.r.config.Context.SetUIntName(&p, &seed, 17))
						original, err = NewNLQSATTacticWithParams(source.r.manager, &p, source.r.config)
						fmMust(t, source.r.config.Context.Del(&p))
					}
					fmMust(t, err)
					defer func() { fmMust(t, original.(interface{ Close() error }).Close()) }()
					target.r.manager.Budget().FailAfter(stop)
					var copy tactic.Tactic
					switch item := original.(type) {
					case *QESATTactic:
						copy, err = item.TranslateWithServices(target.r.manager, target.r.config)
					case *QSATTactic:
						copy, err = item.TranslateWithServices(target.r.manager, target.r.config)
					case *NLQSATTactic:
						copy, err = item.TranslateWithServices(target.r.manager, target.r.config)
					}
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
					fmMust(t, copy.IncRef())
					fmMust(t, copy.DecRef())
				})
			}
			if !success || failures == 0 {
				t.Fatal("did not cover failure and success", failures, success)
			}
		})
	}
}
