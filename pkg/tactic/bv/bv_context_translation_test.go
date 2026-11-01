package bv

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestBVSizeReductionContextTranslationUsesFreshTargetState(t *testing.T) {
	source := bvSizeFixture(t, false)
	goal, _ := bvSizeNativeGoal(t, source, true, true, true)
	var first tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, first.Close()) })
	sharingMust(t, source.Apply(goal, &first))
	if len(source.current.lower) == 0 || len(source.current.upper) == 0 {
		t.Fatal("source bounds were not collected")
	}
	tags := diagnostic.NewTraceTags()
	sharingMust(t, source.BindTrace(tags))
	target := bvSizeFixture(t, false)
	translated, err := source.TranslateWithServices(target.manager, target.context, target.globals, target.services)
	sharingMust(t, err)
	copy := translated.(*BVSizeReductionTactic)
	t.Cleanup(func() { sharingMust(t, copy.Close()) })
	if copy.manager != target.manager || copy.context != target.context || copy.globals != target.globals || copy.services.Cooperation != target.services.Cooperation || copy.trace != tags {
		t.Fatal("translation did not select target services")
	}
	if len(copy.current.lower) != 0 || len(copy.current.upper) != 0 || copy.current == source.current {
		t.Fatal("translation copied live source state")
	}
	sharingMust(t, source.Close())
	next, _ := bvSizeNativeGoal(t, copy, true, true, true)
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	sharingMust(t, copy.Apply(next, &result))
	if result.ModelConverter == nil || copy.current.active == nil {
		t.Fatal("target reduction did not produce a model converter")
	}
}

func TestBVContextTranslationAllocationFrontiers(t *testing.T) {
	for _, kind := range []string{"size", "bounds"} {
		t.Run(kind, func(t *testing.T) {
			success, failures := false, 0
			for stop := int64(0); stop < 128 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					var translated tactic.Tactic
					var err error
					if kind == "size" {
						source, target := bvSizeFixture(t, false), bvSizeFixture(t, false)
						target.manager.Budget().FailAfter(stop)
						translated, err = source.TranslateWithServices(target.manager, target.context, target.globals, target.services)
						target.manager.Budget().FailAfter(-1)
					} else {
						source, target := bvBoundFixture(t), bvBoundFixture(t)
						target.manager.Budget().FailAfter(stop)
						translated, err = source.TranslateWithServices(target.manager, target.context, target.globals, target.services)
						target.manager.Budget().FailAfter(-1)
					}
					var construction memory.ConstructionFailure
					if errors.As(err, &construction) {
						sharingMust(t, construction.Cleanup())
					}
					if err != nil {
						if !errors.Is(err, arena.ErrInjectedFailure) {
							t.Fatal("unexpected translation failure", err)
						}
						failures++
						return
					}
					success = true
					sharingMust(t, translated.IncRef())
					sharingMust(t, translated.DecRef())
				})
			}
			if !success || failures == 0 {
				t.Fatal("did not cover failure and success", failures, success)
			}
		})
	}
}

func TestBVBoundCheckContextTranslationCopiesPolicyAndResetsStatistics(t *testing.T) {
	source := bvBoundFixture(t)
	var first tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, first.Close()) })
	sharingMust(t, source.Apply(bvBoundGoal(t, source, 3, 2), &first))
	if source.stats.Unsats != 1 {
		t.Fatal("source bounds were not checked")
	}
	tags := diagnostic.NewTraceTags()
	sharingMust(t, source.BindTrace(tags))
	target := bvBoundFixture(t)
	translated, err := source.TranslateWithServices(target.manager, target.context, target.globals, target.services)
	sharingMust(t, err)
	copy := translated.(*BVBoundCheckTactic)
	t.Cleanup(func() { sharingMust(t, copy.Close()) })
	if copy.manager != target.manager || copy.context != target.context || copy.globals != target.globals || copy.services.Cooperation != target.services.Cooperation || copy.trace != tags || copy.stats != (BVBoundCheckStats{}) {
		t.Fatal("translation did not create fresh target-owned state")
	}
	key := "bv_ineq_consistency_test_max"
	value, err := target.context.GetUIntName(&copy.params, &key, 0)
	sharingMust(t, err)
	if value != 100 {
		t.Fatal("translated policy", value)
	}
	bvBoundPolicy(t, source, key, 0)
	sharingMust(t, source.Close())
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	sharingMust(t, copy.Apply(bvBoundGoal(t, copy, 3, 2), &result))
	if copy.stats.Unsats != 1 {
		t.Fatal("target bound checker lost copied policy", copy.stats)
	}
}
