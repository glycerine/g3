package bv

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
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

func TestElimSmallBVContextTranslationCopiesPolicy(t *testing.T) {
	source := sharingTacticFixture(t, false)
	sourceGlobals := uparams.BorrowProcessGlobalParamsService()
	original, err := NewElimSmallBVTactic(source.manager, source.context, sourceGlobals, nil, nil)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, original.Close()) })
	p, err := source.context.NewValue()
	sharingMust(t, err)
	key := "max_bits"
	sharingMust(t, source.context.SetUIntName(&p, &key, 2))
	sharingMust(t, original.UpdateParams(&p))
	sharingMust(t, source.context.Del(&p))
	target := sharingTacticFixture(t, false)
	targetGlobals := uparams.BorrowProcessGlobalParamsService()
	cooperation, err := cooperate.NewCooperation(target.manager.Budget())
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, cooperation.Close()) })
	translated, err := original.TranslateWithServices(target.manager, target.context, targetGlobals, cooperation)
	sharingMust(t, err)
	copy := translated.(*ElimSmallBVTactic)
	t.Cleanup(func() { sharingMust(t, copy.Close()) })
	if copy.manager != target.manager || copy.context != target.context || copy.globals != targetGlobals || copy.cooperation != cooperation {
		t.Fatal("target services were not installed")
	}
	value, err := target.context.GetUIntName(&copy.params, &key, 0)
	sharingMust(t, err)
	if value != 2 {
		t.Fatal("parameter snapshot", value)
	}
	sharingMust(t, original.Close())
	sort, err := target.manager.BitVectorSort(2)
	sharingMust(t, err)
	variable, err := target.manager.MkVar(0, sort)
	sharingMust(t, err)
	one, err := target.manager.BitVectorNumeralUnsigned(1, 2)
	sharingMust(t, err)
	body, err := target.manager.MkEq(variable, one)
	sharingMust(t, err)
	q, err := target.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
	sharingMust(t, err)
	goal, err := tactic.NewGoal(target.manager, false, false, false)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, goal.Close()) })
	sharingMust(t, goal.Assert(q))
	second, err := tactic.NewGoal(target.manager, false, false, false)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, second.Close()) })
	sharingMust(t, second.Assert(q))
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	sharingMust(t, copy.Apply(goal, &result))
	formula, err := goal.Form(0)
	sharingMust(t, err)
	if !target.manager.IsFalse(formula) {
		t.Fatal("translated tactic did not enumerate the target quantifier")
	}
	if cooperation.OwnerThread != -1 {
		t.Fatal("disabled cooperation ran a checkpoint")
	}
	sharingMust(t, cooperation.BeginSection())
	sharingMust(t, copy.Apply(second, &result))
	if cooperation.OwnerThread != 0 {
		t.Fatal("enabled cooperation did not run a checkpoint")
	}
	sharingMust(t, cooperation.EndSection())
}

func TestBVArray2UFContextTranslationCreatesIndependentRewriter(t *testing.T) {
	source := newArrayFixture(t)
	selected := source.app(t, ast.ASTArrayOpSelect, source.a, source.i)
	_, err := source.tactic.rewriter.rewrite(selected)
	arrayMust(t, err)
	if len(source.tactic.rewriter.declarations) == 0 {
		t.Fatal("source did not build array declarations")
	}
	tags := diagnostic.NewTraceTags()
	arrayMust(t, source.tactic.BindTrace(tags))
	target := newArrayFixture(t)
	if _, err = source.tactic.TranslateWithServices(target.m, source.tactic.services); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("accepted source services for the target manager", err)
	}
	translated, err := source.tactic.TranslateWithServices(target.m, target.tactic.services)
	arrayMust(t, err)
	copy := translated.(*BVArray2UFTactic)
	t.Cleanup(func() { arrayMust(t, copy.Close()) })
	if copy.manager != target.m || copy.services.Context != target.tactic.services.Context || copy.services.Cooperation != target.tactic.services.Cooperation || copy.trace != tags || copy.rewriter == source.tactic.rewriter || len(copy.rewriter.declarations) != 0 {
		t.Fatal("translation did not create an independent target rewriter")
	}
	arrayMust(t, source.tactic.Close())
	selected = target.app(t, ast.ASTArrayOpSelect, target.a, target.i)
	formula, err := target.m.MkEq(selected, target.v)
	arrayMust(t, err)
	goal := target.goal(t, false, true, false, formula)
	var result tactic.TacticResult
	t.Cleanup(func() { arrayMust(t, result.Close()) })
	arrayMust(t, copy.Apply(goal, &result))
	if result.ModelConverter == nil {
		t.Fatal("translated array rewrite lost its model converter")
	}
}

func TestBVContextTranslationAllocationFrontiers(t *testing.T) {
	for _, kind := range []string{"size", "bounds", "elim", "array"} {
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
					} else if kind == "bounds" {
						source, target := bvBoundFixture(t), bvBoundFixture(t)
						target.manager.Budget().FailAfter(stop)
						translated, err = source.TranslateWithServices(target.manager, target.context, target.globals, target.services)
						target.manager.Budget().FailAfter(-1)
					} else if kind == "elim" {
						base := sharingTacticFixture(t, false)
						source, x := NewElimSmallBVTactic(base.manager, base.context, uparams.BorrowProcessGlobalParamsService(), nil, nil)
						sharingMust(t, x)
						t.Cleanup(func() { sharingMust(t, source.Close()) })
						target := sharingTacticFixture(t, false)
						cooperation, x := cooperate.NewCooperation(target.manager.Budget())
						sharingMust(t, x)
						t.Cleanup(func() { sharingMust(t, cooperation.Close()) })
						target.manager.Budget().FailAfter(stop)
						translated, err = source.TranslateWithServices(target.manager, target.context, uparams.BorrowProcessGlobalParamsService(), cooperation)
						target.manager.Budget().FailAfter(-1)
					} else {
						source, target := newArrayFixture(t), newArrayFixture(t)
						target.m.Budget().FailAfter(stop)
						translated, err = source.tactic.TranslateWithServices(target.m, target.tactic.services)
						target.m.Budget().FailAfter(-1)
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
