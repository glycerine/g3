package fpa

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type fpaTranslationFixture struct {
	manager *ast.ASTManager
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
}

func newFPATranslationFixture(t *testing.T) fpaTranslationFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 30)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	fpaTacticMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	fpaTacticMust(t, err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	fpaTacticMust(t, err)
	context, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget})
	fpaTacticMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	fpaTacticMust(t, err)
	globals, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	fpaTacticMust(t, err)
	fpaTacticMust(t, globals.InitializeSource())
	t.Cleanup(func() {
		fpaTacticMust(t, globals.Close())
		fpaTacticMust(t, catalog.Close())
		fpaTacticMust(t, context.Close())
		fpaTacticMust(t, manager.Close())
		fpaTacticMust(t, numerals.Close())
		fpaTacticMust(t, symbols.Close())
		if budget.Used() != 0 {
			t.Errorf("owner graph uses %d bytes", budget.Used())
		}
	})
	return fpaTranslationFixture{manager, context, globals}
}

func TestFPA2BVContextTranslationCopiesPolicyAndRewrites(t *testing.T) {
	source := newFPATranslationFixture(t)
	p, err := source.context.NewValue()
	fpaTacticMust(t, err)
	key := "hi_fp_unspecified"
	fpaTacticMust(t, source.context.SetBoolName(&p, &key, true))
	original, err := NewFPA2BVTactic(source.manager, source.context, source.globals, &p)
	fpaTacticMust(t, err)
	t.Cleanup(func() { fpaTacticMust(t, original.Close()) })
	fpaTacticMust(t, source.context.Del(&p))
	target := newFPATranslationFixture(t)
	if _, err = original.TranslateWithServices(target.manager, source.context, source.globals); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("accepted source services for target manager", err)
	}
	translated, err := original.TranslateWithServices(target.manager, target.context, target.globals)
	fpaTacticMust(t, err)
	copy := translated.(*FPA2BVTactic)
	t.Cleanup(func() { fpaTacticMust(t, copy.Close()) })
	if copy.manager != target.manager || copy.context != target.context || copy.globals != target.globals || copy.pass == original.pass || copy.pass.converter == original.pass.converter {
		t.Fatal("translation did not create independent target conversion state")
	}
	value, err := target.context.GetBoolName(&copy.params, &key, false)
	fpaTacticMust(t, err)
	if !value {
		t.Fatal("floating-point policy was not translated")
	}
	p, err = source.context.NewValue()
	fpaTacticMust(t, err)
	fpaTacticMust(t, source.context.SetBoolName(&p, &key, false))
	fpaTacticMust(t, original.UpdateParams(&p))
	fpaTacticMust(t, source.context.Del(&p))
	value, err = target.context.GetBoolName(&copy.params, &key, false)
	fpaTacticMust(t, err)
	if !value {
		t.Fatal("source parameter update changed target policy")
	}
	fpaTacticMust(t, original.Close())
	sort, err := target.manager.FloatingPointSort(8, 24)
	fpaTacticMust(t, err)
	name, err := target.manager.Symbols.Intern("translated-float")
	fpaTacticMust(t, err)
	x, err := target.manager.MkConst(name, sort)
	fpaTacticMust(t, err)
	family, err := target.manager.RegisterFloatingPointPlugin()
	fpaTacticMust(t, err)
	formula, err := target.manager.MkFamilyApp(family, int32(ast.FPAIsNaN), nil, []ast.ASTHandle{x})
	fpaTacticMust(t, err)
	goal, err := tactic.NewGoal(target.manager, false, true, false)
	fpaTacticMust(t, err)
	t.Cleanup(func() { fpaTacticMust(t, goal.Close()) })
	fpaTacticMust(t, goal.Assert(formula))
	var result tactic.TacticResult
	t.Cleanup(func() { fpaTacticMust(t, result.Close()) })
	fpaTacticMust(t, copy.Apply(goal, &result))
	if len(result.Goals) != 1 || result.ModelConverter == nil {
		t.Fatal("translated conversion did not publish a model converter")
	}
}

func TestFPA2BVContextTranslationAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 256 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			source := newFPATranslationFixture(t)
			original, err := NewFPA2BVTactic(source.manager, source.context, source.globals, nil)
			fpaTacticMust(t, err)
			t.Cleanup(func() { fpaTacticMust(t, original.Close()) })
			target := newFPATranslationFixture(t)
			target.manager.Budget().FailAfter(stop)
			translated, err := original.TranslateWithServices(target.manager, target.context, target.globals)
			target.manager.Budget().FailAfter(-1)
			injected := errors.Is(err, arena.ErrInjectedFailure)
			var failure memory.Failure
			memout := errors.As(err, &failure) && failure.Kind == memory.FailureMemout
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				fpaTacticMust(t, construction.Cleanup())
			}
			if err != nil {
				if !injected && !memout {
					t.Fatal("unexpected translation failure", err)
				}
				failures++
				return
			}
			success = true
			fpaTacticMust(t, translated.IncRef())
			fpaTacticMust(t, translated.DecRef())
		})
	}
	if !success || failures == 0 {
		t.Fatal("did not reach failure and success", failures, success)
	}
}
