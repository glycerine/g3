package duality

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// The real engine owns independent copies of background theory axioms.
func TestEngineCloseAfterConsumedTheoryRelease(t *testing.T) {
	engineTheoryConsumedRelease(t)
}

func engineTheoryConsumedRelease(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 29)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	config, err := NewConfig(params, nil)
	must(err)
	context, err := NewContext(manager, config)
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	cooperation, err := cooperate.NewCooperation(budget)
	must(err)
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: params, Globals: globals})
	must(err)
	native, err := factory.New(manager, nil, false, true, false, "")
	must(err)
	simplifier, err := interp.NewManagerWithParams(manager, params, globals)
	must(err)
	quantifiers, err := qe.NewLite(manager, nil, true, qe.Config{Context: params, Globals: globals, Cooperation: cooperation})
	must(err)
	source, err := NewRPFP(context)
	must(err)
	scoped, err := numeral.NewScopedInt(numbers.Manager.Z)
	must(err)
	integer, err := scoped.Get()
	must(err)
	words := make([]uint32, 262144)
	words[0], words[len(words)-1] = 17, 1
	must(numbers.Manager.Z.SetLimbs(integer, words))
	value, err := numbers.NewFromScopedInt(scoped)
	must(err)
	parameter, err := manager.NewParameterRational(&value)
	must(err)
	info, err := manager.NewFuncDeclInfo(ast.NullFamilyID, 917, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	label, err := symbols.Intern("prepared-background-consumed")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	expression, err := NewExpr(context, formula)
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	previous, err := NewExpr(context, truth)
	must(err)
	engine, err := NewEngine(source, EngineConfig{Native: native, Auxiliary: native, Factory: factory, Simplifier: simplifier, Quantifiers: quantifiers, Evaluator: model.ModelEvaluatorConfig{Context: params, Globals: globals, Memory: runtime, Cooperation: cooperation}, Theory: []*Expr{previous, expression}})
	must(err)
	must(expression.Close())
	must(previous.Close())
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())
	a, err := runtime.Allocate(8)
	must(err)
	b, err := runtime.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("first boundary", err)
	}
	must(runtime.Deallocate(&a))
	delta = math.MinInt64 + 262144 - runtime.State().GlobalSize + 16
	if err = runtime.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("second boundary", err)
	}
	must(runtime.Deallocate(&b))
	blocks := runtime.Stats().LiveBlocks
	failure := engine.Close()
	if !errors.Is(failure, arena.ErrBounds) || runtime.Stats().LiveBlocks >= blocks {
		t.Fatal("theory cleanup did not fail after native consumption", failure)
	}
	if len(engine.theory) != 1 || engine.theory[0].Closed() {
		t.Fatal("engine did not preserve its unconsumed theory prefix")
	}
	if _, err := engine.Solve(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("partially closed engine accepted solve", err)
	}
	if engine.SetOption("no_conj", "1") || engine.GetCounterexample() != nil || engine.UnwindingGraph() != nil || engine.IsResultRecursionBounded() {
		t.Fatal("partially closed engine remained accessible")
	}

	recovery, err := runtime.Allocate(4 << 20)
	must(err)
	must(engine.Close())
	must(engine.Close())
	must(source.Close())
	must(quantifiers.Close())
	must(simplifier.Close())
	must(native.Close())
	must(factory.Close())
	must(cooperation.Close())
	must(globals.Close())
	must(catalog.Close())
	must(context.Close())
	must(config.Close())
	must(params.Close())
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	if err = runtime.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
		t.Fatal(err)
	}
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("leak", budget.Used())
	}
}
