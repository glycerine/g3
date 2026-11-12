package base

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Drive the real signed TLS accounting lane below its lower bound while the
// first variable's rational declaration parameter is being destroyed
// (~bind_variables releases m_vars front to back). The AST
// reference is consumed even though the physical release reports an error.
func TestVariableBinderCloseRetriesAfterConsumedNativeRelease(t *testing.T) {
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
	binder, err := NewVariableBinder(manager)
	must(err)
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
	must(binder.AddVariable(last))
	must(binder.AddVariable(first))
	consumed, remaining := binder.references[0], binder.references[1]
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
	blocks := runtime.Stats().LiveBlocks
	err = binder.Close()
	if !errors.Is(err, arena.ErrBounds) || binder.closed || !binder.closing || len(binder.references) != 1 || len(binder.variables) != 1 || binder.references[0] != remaining || binder.variables[0] != first {
		t.Fatal("consumed release lost the remaining binder owner", err, binder.closed, binder.closing, binder.references, binder.variables)
	}
	if manager.Stats().LiveReferences+1 != before || runtime.Stats().LiveBlocks >= blocks {
		t.Fatal("last variable release did not consume its reference and native allocation", before, manager.Stats().LiveReferences, blocks, runtime.Stats().LiveBlocks)
	}
	if _, err = manager.RefGet(consumed); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("last variable reference survived its consumed release", err)
	}
	if got, err := manager.RefGet(remaining); err != nil || got != first {
		t.Fatal("remaining variable is no longer live", got, err)
	}
	if err = binder.AddVariable(first); !errors.Is(err, ErrBindVariablesOwner) {
		t.Fatal("closing binder accepted registration", err)
	}
	if result, err := binder.Bind(first, true); !errors.Is(err, ErrBindVariablesOwner) || result != (ast.ASTRef{}) {
		t.Fatal("closing binder accepted abstraction", result, err)
	}

	recovery, err := runtime.Allocate(400000)
	must(err)
	state, blocks = runtime.State(), runtime.Stats().LiveBlocks
	must(binder.Close())
	if !binder.closed || binder.closing || len(binder.references) != 0 || len(binder.variables) != 0 || manager.Stats().LiveReferences+2 != before {
		t.Fatal("close retry did not drain the remaining binder variable")
	}
	if _, err = manager.RefGet(remaining); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("close retry did not consume the remaining reference", err)
	}
	must(binder.Close())
	if runtime.State() != state || runtime.Stats().LiveBlocks != blocks {
		t.Fatal("close retry replayed the consumed native release")
	}
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
	finish(manager.Close)
	finish(numbers.Close)
	finish(symbols.Close)
	if err = runtime.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
		t.Fatal(err)
	}
	finish(runtime.Close)
	if budget.Used() != 0 {
		t.Fatal("binder cleanup leaked budget", budget.Used())
	}
}
