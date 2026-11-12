package fp

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// The context's binder member must survive a later physical cleanup error,
// even after the engine's initial AST admission guard has succeeded. Drive the
// native TLS accounting lane below its lower bound while the first variable's
// rational declaration parameter is destroyed (~bind_variables releases m_vars
// front to back); the reference is consumed but the binder's following
// variable still needs a later engine Close retry.
func TestEngineClosePreservesBinderAfterConsumedNativeRelease(t *testing.T) {
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
	binder := engine.binder
	relations := engine.relations
	background := engine.background
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
			t.Fatal("engine binder cleanup leaked budget", budget.Used())
		}
	})
	boolean, err := manager.MkBoolSort()
	must(err)
	first, err := manager.MkFreshConst("binder-before-consumed-release", boolean)
	must(err)

	scoped, err := numeral.NewScopedInt(numbers.Manager.Z)
	must(err)
	integer, err := scoped.Get()
	must(err)
	digits := make([]uint32, 32768)
	digits[0], digits[len(digits)-1] = 17, 1
	must(numbers.Manager.Z.SetLimbs(integer, digits))
	value, err := numbers.NewFromScopedInt(scoped)
	must(err)
	parameter, err := manager.NewParameterRational(&value)
	must(err)
	info, err := manager.NewFuncDeclInfo(ast.NullFamilyID, 917, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
	must(err)
	name, err := symbols.Intern("binder-consumed-native-release")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(name, nil, boolean, info)
	must(err)
	last, err := manager.MkApp(declaration, nil)
	must(err)
	must(engine.RegisterVariable(last))
	must(engine.RegisterVariable(first))
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())
	if count, err := manager.GetRefCount(declaration); err != nil || count != 1 {
		t.Fatal("last variable is not the sole declaration owner", count, err)
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
	before := manager.Stats().LiveReferences
	// The rule manager's label rewriter (rule_manager::m_rwr) holds three
	// scratch references for the engine's life, as does the relation
	// manager's context var_subst (context::m_var_subst).
	if before != baseline+9 {
		t.Fatal("fixture does not have only the background and two binder references and the label rewriter and var_subst scratch", baseline, before)
	}
	blocks := runtime.Stats().LiveBlocks
	err = engine.Close()
	if !errors.Is(err, arena.ErrBounds) || engine.closed || !engine.closing || engine.binder != binder {
		t.Fatal("engine discarded its interrupted binder", err, engine.closed, engine.closing, engine.binder == binder)
	}
	if engine.relations != relations || engine.background != background {
		t.Fatal("engine disposed later owners after its binder failed")
	}
	// ~context destroys m_bind_variables before m_rule_manager, so the
	// label rewriter's three scratch references are still held.
	if manager.Stats().LiveReferences+1 != before || runtime.Stats().LiveBlocks >= blocks {
		t.Fatal("binder failure did not consume exactly one AST reference and a native allocation", before, manager.Stats().LiveReferences, blocks, runtime.Stats().LiveBlocks)
	}
	if _, err = manager.GetRefCount(last); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("last variable survived its consumed release", err)
	}
	if count, err := manager.GetRefCount(first); err != nil || count != 1 {
		t.Fatal("binder lost the remaining variable owner", count, err)
	}
	if err = engine.RegisterVariable(first); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("closing engine accepted variable registration", err)
	}
	if err = engine.SetBackground(first); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("closing engine accepted background replacement", err)
	}
	if _, err = engine.QueryExpression(first); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("closing engine accepted a query", err)
	}
	if result, err := binder.Bind(first, true); !errors.Is(err, base.ErrBindVariablesOwner) || result != (ast.ASTRef{}) {
		t.Fatal("interrupted binder accepted abstraction", result, err)
	}

	recovery, err = runtime.Allocate(400000)
	must(err)
	state, blocks = runtime.State(), runtime.Stats().LiveBlocks
	must(engine.Close())
	if !engine.closed || engine.binder != nil || engine.relations != nil || engine.background != (ast.ASTRef{}) || manager.Stats().LiveReferences != baseline {
		t.Fatal("engine retry did not finish the binder and later owners", engine.closed, engine.binder, engine.relations, engine.background, baseline, manager.Stats().LiveReferences)
	}
	if _, err = manager.GetRefCount(first); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("engine retry left the remaining binder variable live", err)
	}
	must(engine.Close())
	must(binder.Close())
	if runtime.State() != state || runtime.Stats().LiveBlocks != blocks || manager.Stats().LiveReferences != baseline {
		t.Fatal("retry replayed the consumed native release", state, runtime.State(), blocks, runtime.Stats().LiveBlocks, baseline, manager.Stats().LiveReferences)
	}
	// The enclosing manager stays live: engine cleanup must release its own
	// roots without relying on the manager's eventual arena teardown.
	_, err = manager.ProofMode()
	must(err)
}
