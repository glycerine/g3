package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var errSpacerEvaluatorConstruction = errors.New("spacer evaluator construction failed")

func TestSpacerModelEvaluatorFailedConstructionPreservesRefusedOwner(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var evaluator *model.ModelEvaluator
	var config model.ModelEvaluatorConfig
	var failure error
	active, callbacks, intact := false, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active {
			callbacks++
			if failure == nil {
				returned, cause := closeFailedSpacerModelEvaluator(evaluator, config, errSpacerEvaluatorConstruction)
				failure = cause
				intact = returned == nil && errors.Is(cause, errSpacerEvaluatorConstruction) && errors.Is(cause, ast.ErrASTPending)
			}
			var cleanup memory.ConstructionFailure
			if errors.As(failure, &cleanup) {
				intact = intact && errors.Is(cleanup.Cleanup(), ast.ErrASTPending)
			}
		}
		return source
	}})
	must(err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	must(err)
	params, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	cooperation, err := cooperate.NewCooperation(budget)
	must(err)
	md, err := model.NewModel(manager)
	must(err)
	config = model.ModelEvaluatorConfig{Context: params, Globals: globals, Memory: runtime, Cooperation: cooperation}
	evaluator, err = model.NewModelEvaluator(md.Core(), config)
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if cleanup, ok := failure.(memory.ConstructionFailure); ok {
			must(cleanup.Cleanup())
		} else if evaluator != nil {
			must(evaluator.Close())
		}
		must(md.Close())
		must(cooperation.Close())
		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numerals.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("evaluator construction pending fixture leaked bytes", budget.Used())
		}
	}()
	value, err := numerals.NewValue()
	must(err)
	defer func() { must(numerals.Del(&value)) }()
	must(numerals.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("construction cleanup did not preserve evaluator", body, callbacks, intact, failure)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime || cleanup.CleanupCause() == nil {
		t.Fatal("missing evaluator cleanup receipt", failure)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if cleanup.CleanupCause() != nil {
		t.Fatal("evaluator cleanup retry remained pending", cleanup.CleanupCause())
	}
	evaluator, failure = nil, nil
}
