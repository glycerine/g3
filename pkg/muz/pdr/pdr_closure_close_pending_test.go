package pdr

// Source: convex_closure lifetime in src/muz/pdr/pdr_closure.cpp.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestConvexClosureClosePreservesRootsDuringNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var closure *convexClosure
	var closeErr error
	active, callback := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && !callback {
			callback = true
			root := closure.references[0]
			closeErr = closure.Close()
			if !errors.Is(closeErr, ast.ErrASTPending) || !closure.closing || closure.closed || closure.manager == nil || len(closure.references) != 1 || closure.references[0] != root {
				t.Fatalf("refused convex-closure cleanup lost its root: %v", closeErr)
			}
			if err := closure.open(); !errors.Is(err, ast.ErrASTPending) {
				t.Fatalf("closing convex closure accepted use: %v", err)
			}
		}
		return source
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	boolean, err := manager.MkBoolSort()
	must(err)
	root, err := manager.MkFreshConst("pending-convex-closure", boolean)
	must(err)
	reference, err := manager.NewRef(root)
	must(err)
	closure = &convexClosure{manager: manager, transformer: &PredicateTransformer{}, references: []ast.ASTRef{reference}}

	value, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || !callback || !errors.Is(closeErr, ast.ErrASTPending) || len(closure.references) != 1 {
		t.Fatal("native guard did not refuse convex-closure cleanup", body, callback, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(closure.Close())
	must(closure.Close())
	if !closure.closed || closure.closing || closure.manager != nil || closure.transformer != nil || len(closure.references) != 0 {
		t.Fatal("convex-closure cleanup retry did not finish")
	}

	must(numbers.Del(&value))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("convex-closure cleanup fixture leaked bytes", budget.Used())
	}
}
