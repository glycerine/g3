package fp

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source dl_context.cpp:344-346 owns registered declarations in context::m_pinned;
// Go stores those context roots in RuleManager.predicates. Infinite-domain
// predicates have no finite-table declaration owner, exposing that lifetime.
// Give both declarations distinct large rational parameters so either map
// iteration order reaches a native release error with one predicate remaining.
func TestEngineClosePreservesRuleManagerAfterConsumedNativeRelease(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	baseline := manager.Stats().LiveReferences
	engine, err := NewPropositionalEngine(manager)
	must(err)
	rules, relations, background := engine.rules, engine.relations, engine.background
	var recovery memory.Block
	t.Cleanup(func() {
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
			t.Fatal("cleanup retry limit")
		}
		finish(engine.Close)
		finish(manager.Close)
		finish(numbers.Close)
		finish(symbols.Close)
		if recovery != (memory.Block{}) {
			if err := runtime.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
				t.Fatal("recovery storage cleanup", err)
			}
		}
		finish(runtime.Close)
		if budget.Used() != 0 {
			t.Fatal("engine rule manager cleanup leaked budget", budget.Used())
		}
	})
	boolean, err := manager.MkBoolSort()
	must(err)
	family, err := manager.RegisterArithmeticPlugin()
	must(err)
	domain, err := manager.MkFamilySort(family, 1, nil)
	must(err)
	var declarations []ast.ASTHandle
	for i := 0; i < 2; i++ {
		scoped, err := numeral.NewScopedInt(numbers.Manager.Z)
		must(err)
		integer, err := scoped.Get()
		must(err)
		digits := make([]uint32, 32768)
		digits[0], digits[len(digits)-1] = uint32(17+i), 1
		must(numbers.Manager.Z.SetLimbs(integer, digits))
		value, err := numbers.NewFromScopedInt(scoped)
		must(err)
		parameter, err := manager.NewParameterRational(&value)
		must(err)
		info, err := manager.NewFuncDeclInfo(ast.NullFamilyID, 917, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
		must(err)
		name, err := symbols.Intern(fmt.Sprintf("engine-rule-manager-native-%d", i))
		must(err)
		declaration, err := manager.MkFuncDeclWithInfo(name, []ast.ASTHandle{domain}, boolean, info)
		must(err)
		must(engine.RegisterPredicate(declaration))
		declarations = append(declarations, declaration)
		must(manager.DelFuncDeclInfo(&info))
		must(manager.DelParameter(&parameter))
		must(numbers.Del(&value))
		must(scoped.Close())
		if count, err := manager.GetRefCount(declaration); err != nil || count != 1 {
			t.Fatal("rule manager is not the sole predicate owner", count, err)
		}
	}

	firstBlock, err := runtime.Allocate(8)
	must(err)
	secondBlock, err := runtime.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&firstBlock, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("first signed accounting adjustment", err)
	}
	must(runtime.Deallocate(&firstBlock))
	state := runtime.State()
	delta = math.MinInt64 + 1000 - state.GlobalSize + 16
	if err = runtime.Reallocate(&secondBlock, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("second signed accounting adjustment", err)
	}
	must(runtime.Deallocate(&secondBlock))
	if state = runtime.State(); state.GlobalSize != math.MinInt64+1000 || state.LocalSize != 0 {
		t.Fatal("failed real TLS accounting setup", state)
	}
	before, blocks := manager.Stats().LiveReferences, runtime.Stats().LiveBlocks
	// The rule manager's label rewriter (rule_manager::m_rwr) holds three
	// scratch references for the engine's life, as does the relation
	// manager's context var_subst (context::m_var_subst).
	if before != baseline+9 {
		t.Fatal("fixture does not have only background and two predicate references and the label rewriter and var_subst scratch", baseline, before)
	}
	err = engine.Close()
	if !errors.Is(err, arena.ErrBounds) || engine.closed || !engine.closing || engine.rules != rules {
		t.Fatal("engine discarded its interrupted rule manager", err, engine.closed, engine.closing, engine.rules == rules)
	}
	if engine.relations != relations || engine.background != background {
		t.Fatal("engine disposed later owners after its rule manager failed")
	}
	// ~context destroys m_bind_variables before m_rule_manager.
	if engine.binder != nil {
		t.Fatal("engine kept its binder past the rule manager's failure")
	}
	// The label rewriter's three scratch references go first (m_rwr is
	// destroyed before the predicate table), then the consumed predicate.
	if manager.Stats().LiveReferences+4 != before || runtime.Stats().LiveBlocks+1 != blocks {
		t.Fatal("failure did not consume exactly one predicate reference and native allocation", before, manager.Stats().LiveReferences, blocks, runtime.Stats().LiveBlocks)
	}
	live, consumed := 0, 0
	var survivor ast.ASTHandle
	for _, declaration := range declarations {
		count, err := manager.GetRefCount(declaration)
		switch {
		case errors.Is(err, ast.ErrASTHandle) || err == nil && count == 0:
			// A consumed declaration can still have zero-count placement owned
			// by the AST manager's interrupted destruction state.
			consumed++
		case err == nil && count == 1:
			live++
			survivor = declaration
		default:
			t.Fatal("unexpected predicate ownership after native error", count, err)
		}
	}
	if live != 1 || consumed != 1 {
		t.Fatal("rule manager lost the unvisited predicate", live, consumed)
	}
	truth, err := manager.RefGet(background)
	must(err)
	if !manager.IsTrue(truth) {
		t.Fatal("interrupted close changed the background")
	}
	if err = engine.RegisterPredicate(survivor); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("closing engine accepted predicate registration", err)
	}
	if err = engine.SetBackground(truth); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("closing engine accepted background replacement", err)
	}
	if _, err = engine.QueryExpression(truth); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("closing engine accepted a query", err)
	}

	recovery, err = runtime.Allocate(400000)
	must(err)
	blocks = runtime.Stats().LiveBlocks
	must(engine.Close())
	if !engine.closed || engine.rules != nil || engine.binder != nil || engine.relations != nil || engine.background != (ast.ASTRef{}) || manager.Stats().LiveReferences != baseline {
		t.Fatal("engine retry did not finish predicate and later owners", engine.closed, engine.rules, engine.binder, engine.relations, engine.background, baseline, manager.Stats().LiveReferences)
	}
	if manager.IsLiveHandle(survivor) {
		t.Fatal("engine retry left the unvisited predicate live")
	}
	if runtime.Stats().LiveBlocks+1 != blocks {
		t.Fatal("engine retry did not release exactly the remaining native allocation", blocks, runtime.Stats().LiveBlocks)
	}
	state, blocks = runtime.State(), runtime.Stats().LiveBlocks
	must(engine.Close())
	must(engine.Close())
	if runtime.State() != state || runtime.Stats().LiveBlocks != blocks || manager.Stats().LiveReferences != baseline {
		t.Fatal("repeated close replayed a consumed native release", state, runtime.State(), blocks, runtime.Stats().LiveBlocks)
	}
	// Verify the engine drained its own references before enclosing teardown.
	_, err = manager.ProofMode()
	must(err)
}
