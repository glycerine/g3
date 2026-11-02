package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type sparseIntervalNestedCleanupFailure struct {
	original error
	owner    *memory.Runtime
	cleaned  int
}

func (f *sparseIntervalNestedCleanupFailure) Error() string                 { return f.original.Error() }
func (f *sparseIntervalNestedCleanupFailure) Unwrap() error                 { return f.original }
func (f *sparseIntervalNestedCleanupFailure) RuntimeOwner() *memory.Runtime { return f.owner }
func (f *sparseIntervalNestedCleanupFailure) OriginalCause() error          { return f.original }
func (*sparseIntervalNestedCleanupFailure) CleanupCause() error             { return nil }
func (f *sparseIntervalNestedCleanupFailure) Cleanup() error {
	f.cleaned++
	return nil
}

func TestSparseIntervalConstructionReceiptRetriesNativeRefusalAndNestedCleanup(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	active := false
	var store *SparseIntervalDatalogStore
	var nested *sparseIntervalNestedCleanupFailure
	var construction error
	native, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && construction == nil {
			construction = finishSparseIntervalStoreConstruction(nested, store)
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: native, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(native.Close())
		if budget.Used() != 0 {
			t.Fatal("sparse interval cleanup fixture leaked bytes", budget.Used())
		}
	}()
	must(manager.RegisterDeclarationPlugins())
	family, err := manager.RegisterArithmeticPlugin()
	must(err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("sparse-interval-cleanup")
	must(err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	external, err := manager.NewRef(declaration)
	must(err)
	defer func() { must(manager.DelRef(&external)) }()
	before, err := manager.GetRefCount(declaration)
	must(err)
	store, err = NewSparseIntervalDatalogStore(manager)
	must(err)
	must(store.Register(declaration))
	nested = &sparseIntervalNestedCleanupFailure{original: arena.ErrInjectedFailure, owner: native}

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(native.SetMaxCount(uint64(native.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(native.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	var failure memory.ConstructionFailure
	if body == nil || !errors.As(construction, &failure) || failure.RuntimeOwner() != native || !errors.Is(failure.CleanupCause(), ast.ErrASTPending) {
		t.Fatal("native sparse/interval cleanup refusal lost retry receipt", body, construction)
	}
	if nested.cleaned != 1 {
		t.Fatal("outer store cleanup ran before its nested construction owner", nested.cleaned)
	}
	entry := store.entries[declaration]
	if entry == nil || entry.root == (ast.ASTRef{}) {
		t.Fatal("refused store cleanup lost consumed declaration root")
	}
	if _, err = manager.RefGet(entry.root); err != nil {
		t.Fatal("refused store cleanup invalidated declaration root", err)
	}
	must(failure.Cleanup())
	if nested.cleaned != 1 || failure.CleanupCause() != nil || !store.closed {
		t.Fatal("retry did not clean nested receipt and concrete store", nested.cleaned, failure.CleanupCause(), store.closed)
	}
	after, err := manager.GetRefCount(declaration)
	must(err)
	if after != before {
		t.Fatal("store cleanup retry left declaration pin", before, after)
	}
}

func TestSparseIntervalConstructionHelpersReturnOriginalAfterCleanClose(t *testing.T) {
	original := arena.ErrInjectedFailure
	if err := finishSparseIntervalStoreConstruction(original, nil); !errors.Is(err, original) {
		t.Fatal("nil store changed construction cause", err)
	}
	if err := finishSparseIntervalEvaluatorConstruction(original, nil); !errors.Is(err, original) {
		t.Fatal("nil evaluator changed construction cause", err)
	}
	if err := finishSparseIntervalActivationConstruction(original, nil); !errors.Is(err, original) {
		t.Fatal("nil activation changed construction cause", err)
	}
	if err := finishSparseIntervalEntriesConstruction(original, nil); !errors.Is(err, original) {
		t.Fatal("empty staged entries changed construction cause", err)
	}
}
