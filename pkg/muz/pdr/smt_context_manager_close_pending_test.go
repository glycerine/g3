package pdr

// Source: smt_context_manager destruction in
// src/muz/pdr/pdr_smt_context_manager.cpp.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTContextManagerCloseRetriesGuardPredicateRelease(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var contexts *SMTContextManager
	var closeErr error
	active, callback := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && !callback {
			callback = true
			root := contexts.predicates[0]
			closeErr = contexts.Close()
			if !errors.Is(closeErr, ast.ErrASTPending) || contexts.closed || contexts.manager == nil || len(contexts.predicates) != 1 || contexts.predicates[0] != root {
				t.Fatalf("refused SMT-context manager cleanup lost its guard: %v", closeErr)
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
	parameters, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	contexts, err = NewSMTContextManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: parameters})
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	guard, err := manager.MkFreshConst("pending-smt-context-guard", boolean)
	must(err)
	guardRef, err := manager.NewRef(guard)
	must(err)
	contexts.predicates = append(contexts.predicates, guardRef)

	value, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || !callback || !errors.Is(closeErr, ast.ErrASTPending) || contexts.predicates[0] != guardRef {
		t.Fatal("native guard did not preserve SMT-context manager cleanup", body, callback, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(contexts.Close())
	must(contexts.Close())
	if !contexts.closed || contexts.manager != nil || len(contexts.predicates) != 0 {
		t.Fatal("SMT-context manager cleanup retry did not finish")
	}

	must(numbers.Del(&value))
	must(parameters.Close())
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("SMT-context manager cleanup fixture leaked bytes", budget.Used())
	}
}
