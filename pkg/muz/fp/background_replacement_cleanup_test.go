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
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source: spacer_manager.h set_background assigns an owning expr_ref, taking
// the new reference before destroying the old background. A native cleanup
// failure must keep that replacement owned until publication can finish.
func TestSpacerBackgroundReplacementResumesNativeCleanup(t *testing.T) {
	for _, completion := range []string{"setter", "query", "property", "replacement", "close"} {
		t.Run(completion, func(t *testing.T) { testSpacerBackgroundReplacementCleanup(t, completion) })
	}
}

func testSpacerBackgroundReplacementCleanup(t *testing.T, completion string) {
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
			t.Fatal("background replacement leaked storage", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("native-spacer-background-replacement-P")
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(predicate))
	fact, err := manager.MkApp(predicate, nil)
	must(err)
	must(engine.addRuleEager(fact, "native-spacer-background-replacement-fact"))

	// A completed expression query also owns its background. Retire that clone
	// before replacement, leaving the parent as the sole large-numeral owner.
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
	label, err := symbols.Intern("native-spacer-background-replacement")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	must(engine.SetBackground(formula))
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())
	status, err := engine.QueryExpression(fact)
	must(err)
	if status != lbool.True || engine.queryClone == nil || engine.queryClone.spacer == nil {
		t.Fatal("missing completed query clone", status)
	}
	must(engine.clearQueryResources())
	replacement, err := manager.MkFalse()
	must(err)
	replacementRoot, err := manager.NewRef(replacement)
	must(err)
	defer func() { must(manager.DelRef(&replacementRoot)) }()
	replacementRefs, err := manager.GetRefCount(replacement)
	must(err)
	revision := engine.revision
	if refs, err := manager.GetRefCount(formula); err != nil || refs != 1 {
		t.Fatal("parent not sole background owner", refs, err)
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
	stopped := engine.SetBackground(replacement)
	if !errors.Is(stopped, arena.ErrBounds) || runtime.Stats().LiveBlocks >= blocks {
		t.Fatal("no actual background release failure", stopped, blocks, runtime.Stats().LiveBlocks)
	}
	pending := engine.pendingBackground
	if pending == (ast.ASTRef{}) || engine.background != (ast.ASTRef{}) || engine.revision != revision || engine.queryReady {
		t.Fatal("failed replacement lost ownership or published a background/result", engine.background, pending, revision, engine.revision, engine.queryReady)
	}
	incoming, err := manager.RefGet(pending)
	must(err)
	refs, err := manager.GetRefCount(replacement)
	must(err)
	if incoming != replacement || refs != replacementRefs+1 {
		t.Fatal("pending replacement does not own exactly one incoming reference", incoming, refs, replacementRefs)
	}
	recovery, err = runtime.Allocate(4 << 20)
	must(err)
	expected, expectedRevision := lbool.False, revision+1
	switch completion {
	case "setter":
		must(engine.SetBackground(replacement))
		must(engine.SetBackground(replacement))
	case "query":
		status, err = engine.QueryExpression(fact)
		must(err)
		if status != lbool.False {
			t.Fatal("query did not finish the false-background update", status)
		}
	case "property":
		_, err = engine.NumLevels(predicate)
		must(err)
	case "replacement":
		truth, err := manager.MkTrue()
		must(err)
		must(engine.SetBackground(truth))
		expected, expectedRevision = lbool.True, revision+2
	case "close":
		must(engine.Close())
		must(engine.Close())
		if engine.pendingBackground != (ast.ASTRef{}) || engine.background != (ast.ASTRef{}) || !engine.closed {
			t.Fatal("close lost the pending replacement")
		}
		refs, err := manager.GetRefCount(replacement)
		must(err)
		if refs != replacementRefs {
			t.Fatal("close leaked incoming background ownership", refs, replacementRefs)
		}
		return
	}
	if engine.pendingBackground != (ast.ASTRef{}) || engine.revision != expectedRevision {
		t.Fatal("replacement was not published exactly once", engine.pendingBackground, engine.revision, expectedRevision)
	}
	if completion != "query" {
		if _, err := engine.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
			t.Fatal("replacement exposed the stale result", err)
		}
	}
	status, err = engine.QueryExpression(fact)
	must(err)
	if status != expected {
		t.Fatal("fresh query did not use the completed background", status, expected)
	}
	must(engine.Close())
	must(engine.Close())
	refs, err = manager.GetRefCount(replacement)
	must(err)
	if refs != replacementRefs {
		t.Fatal("engine close leaked replacement ownership", refs, replacementRefs)
	}
}

func TestBackgroundReplacementReferenceFailureKeepsQuerySnapshot(t *testing.T) {
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("background allocation fixture leaked storage", budget.Used())
		}
	})
	f := newPDRLifecycleFixture(t)
	engine, manager := f.engine, f.manager
	budget = manager.Budget()
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(engine.SelectEngine("spacer"))
	query := f.declarations["q"]
	status, err := engine.QueryPredicate(query)
	must(err)
	if status != lbool.True {
		t.Fatal("initial query did not reach its fact", status)
	}
	replacement, err := manager.MkFalse()
	must(err)
	var pressure []ast.ASTRef
	defer func() {
		budget.FailAfter(-1)
		must(closeASTRefs(manager, pressure))
	}()
	budget.FailAfter(0)
	for {
		reference, cause := manager.NewRef(ast.ASTHandle{})
		if cause != nil {
			if reference != (ast.ASTRef{}) || !errors.Is(cause, arena.ErrInjectedFailure) {
				t.Fatal("reference page frontier", reference, cause)
			}
			break
		}
		pressure = append(pressure, reference)
	}
	before, revision, old := manager.Stats(), engine.revision, engine.background
	cause := engine.SetBackground(replacement)
	budget.FailAfter(-1)
	if !errors.Is(cause, arena.ErrInjectedFailure) || engine.pendingBackground != (ast.ASTRef{}) || engine.background != old || engine.revision != revision {
		t.Fatal("unowned replacement changed its source state", cause, engine.background, old, engine.revision, revision)
	}
	after := manager.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
		t.Fatal("failed owner allocation changed live AST counts", before, after)
	}
	status, err = engine.LastResult()
	must(err)
	if status != lbool.True {
		t.Fatal("unstarted background replacement invalidated the query", status)
	}
	must(closeASTRefs(manager, pressure))
	pressure = nil
	must(engine.SetBackground(replacement))
	status, err = engine.QueryPredicate(query)
	must(err)
	if status != lbool.False {
		t.Fatal("retry did not install the false background", status)
	}
	must(engine.Close())
	must(engine.Close())
}
