package fp

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source: virtual_solver::~virtual_solver disables its guard before releasing
// member roots. A stopped native release must resume without replaying either.
func TestSpacerExpressionCloneCleanupResumesNativeCoverRelease(t *testing.T) {
	for _, completion := range []string{"clear", "statistics-reset", "parameter-update", "query-retry", "close"} {
		t.Run(completion, func(t *testing.T) { testSpacerExpressionCloneCoverCleanup(t, completion) })
	}
}

func testSpacerExpressionCloneCoverCleanup(t *testing.T, completion string) {
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	budget := arena.NewBudget(1 << 29)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	engine, err := NewPropositionalEngineWithServices(manager, params, globals)
	must(err)
	must(engine.SelectEngine("spacer"))
	for _, setting := range []string{"fixedpoint.xform.slice", "fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
		must(globals.SourceSetName(setting, "false"))
	}
	var recovery memory.Block
	defer func() {
		finish := func(close func() error) {
			t.Helper()
			for i := 0; i < 128; i++ {
				before := runtime.Stats().LiveBlocks
				err := close()
				if err == nil {
					return
				}
				if !errors.Is(err, arena.ErrBounds) || runtime.Stats().LiveBlocks >= before {
					t.Fatal("cleanup made no physical progress", err, runtime.State())
				}
			}
			t.Fatal("cleanup horizon")
		}
		finish(engine.Close)
		finish(manager.Close)
		finish(globals.Close)
		finish(catalog.Close)
		finish(params.Close)
		finish(numbers.Close)
		finish(symbols.Close)
		if recovery != (memory.Block{}) {
			if err := runtime.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
				t.Fatal("recovery storage cleanup", err)
			}
		}
		finish(runtime.Close)
		if budget.Used() != 0 {
			t.Fatal("expression clone cleanup leaked storage", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("native-spacer-clone-cover-P")
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(predicate))
	fact, err := manager.MkApp(predicate, nil)
	must(err)
	must(engine.AddRule(fact, "native-spacer-clone-cover-fact"))

	status, err := engine.QueryExpression(fact)
	must(err)
	if status != lbool.True || engine.queryClone == nil || engine.queryClone.spacer == nil {
		t.Fatal("missing solved clone", status)
	}
	clone := engine.queryClone
	// Add a cover after solving, so its assertion remains buffered in the
	// virtual solver. Closing the frames first leaves the solver with the last
	// root of this large rational declaration parameter.
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
	label, err := symbols.Intern("native-spacer-clone-cover")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	must(engine.AddCover(3, predicate, formula))
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())
	refs, err := manager.GetRefCount(formula)
	must(err)
	if refs < 2 {
		t.Fatal("cover is not owned by its frame and solver", refs)
	}
	countChecks := func(owner *PropositionalEngine) uint32 {
		t.Helper()
		current := statistics.New()
		must(owner.CollectStatistics(current))
		var total uint32
		for i := 0; i < current.Size(); i++ {
			if current.IsUint(i) && current.Key(i) == "virtual_solver.checks" {
				total += current.UintValue(i)
			}
		}
		return total
	}
	checks := countChecks(engine)
	if checks == 0 {
		t.Fatal("query did not record solver checks")
	}

	// Reach a native deallocation error after a physical block is freed.
	a, err := runtime.Allocate(8)
	must(err)
	b, err := runtime.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("first TLS boundary", err)
	}
	must(runtime.Deallocate(&a))
	delta = math.MinInt64 + 262144 - runtime.State().GlobalSize + 16
	if err = runtime.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("second TLS boundary", err)
	}
	must(runtime.Deallocate(&b))
	if state := runtime.State(); state.GlobalSize != math.MinInt64+262144 || state.LocalSize != 0 {
		t.Fatal("native TLS fixture", state)
	}
	blocks := runtime.Stats().LiveBlocks
	stoppedStatus, stopped := engine.QueryExpression(fact)
	if stoppedStatus != lbool.Undef || !errors.Is(stopped, arena.ErrBounds) || runtime.Stats().LiveBlocks >= blocks {
		t.Fatal("query did not stop after native clone storage was released", stoppedStatus, stopped, blocks, runtime.Stats().LiveBlocks)
	}
	pending := engine.queryCloneDisposal
	if engine.queryClone != clone || clone.closed || !clone.closing || pending == nil || pending.clone != clone || !pending.statisticsReady {
		t.Fatal("native cleanup discarded its clone or complete statistics")
	}
	if _, err := engine.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("cleanup failure published a result", err)
	}
	if _, err := engine.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("cleanup failure published an answer", err)
	}
	if got := countChecks(engine); got != checks {
		t.Fatal("pending cleanup changed checks", got, checks)
	}
	recovery, err = runtime.Allocate(4 << 20)
	must(err)
	switch completion {
	case "clear":
		must(engine.clearQueryResources())
	case "statistics-reset":
		must(engine.ResetStatistics())
		checks = 0
	case "parameter-update":
		must(engine.SetQueryParameters(nil))
		checks = 0
	case "query-retry":
		status, err = engine.QueryExpression(fact)
		must(err)
		if status != lbool.True {
			t.Fatal("query retry lost the original fact", status)
		}
		checks += countChecks(engine.queryClone)
	case "close":
		must(engine.Close())
		must(engine.Close())
		if !clone.closed || engine.queryCloneDisposal != nil {
			t.Fatal("engine close lost its pending clone")
		}
		return
	}
	if !clone.closed || engine.queryCloneDisposal != nil {
		t.Fatal("cleanup did not finish its original clone")
	}
	if got := countChecks(engine); got != checks {
		t.Fatal("cleanup completion changed checks", got, checks)
	}
	must(engine.clearQueryResources())
	must(engine.clearQueryResources())
	if got := countChecks(engine); got != checks {
		t.Fatal("repeated cleanup duplicated checks", got, checks)
	}
	status, err = engine.QueryExpression(fact)
	must(err)
	if status != lbool.True {
		t.Fatal("fresh query lost the original fact", status)
	}
	if got := countChecks(engine); got != checks+countChecks(engine.queryClone) {
		t.Fatal("fresh query lost archived checks", got, checks)
	}
}
