package clp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestCLPLocalCleanupOwnersSurviveNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var refuse func() error
	var failure error
	active := false
	refusing := false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && failure == nil && !refusing {
			refusing = true
			failure = refuse()
			refusing = false
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("fixture leaked bytes", budget.Used())
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	variable, err := manager.MkVar(0, boolean)
	must(err)
	fresh, err := manager.MkFreshConst("clp-local-owner", boolean)
	must(err)
	sub, err := rewriter.NewVarSubstitution(manager, false)
	must(err)
	output, err := manager.NewRef(ast.ASTHandle{})
	must(err)
	must(sub.Substitute(variable, []ast.ASTHandle{fresh}, output))
	originalOutput := output
	environment := &groundEnvironment{manager: manager}
	refuse = func() error {
		cause := environment.finishSubstitution(sub, &output, nil)
		return errors.Join(cause, environment.dispose())
	}

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	trigger := func() error {
		t.Helper()
		failure = nil
		must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
		var invocation ast.ASTArithmeticNumeralInvocation
		active = true
		_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
		active = false
		must(runtime.SetMaxCount(0))
		must(manager.CleanupArithmeticNumeralInvocation(&invocation))
		if body == nil {
			t.Fatal("did not enter native failure transport")
		}
		return failure
	}

	failure = trigger()
	if !errors.Is(failure, ast.ErrASTPending) || output != (ast.ASTRef{}) || environment.substitution != sub || len(environment.temporaries) != 1 || environment.temporaries[0] != originalOutput {
		t.Fatal("refused substitution cleanup lost its owners", failure)
	}
	if _, err := manager.RefGet(environment.temporaries[0]); err != nil {
		t.Fatal("refused substitution cleanup invalidated output", err)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
		t.Fatal("substitution cleanup is not retryable", failure)
	}
	must(cleanup.Cleanup())
	if environment.substitution != nil || environment.temporaries != nil || cleanup.CleanupCause() != nil {
		t.Fatal("substitution cleanup retry did not retire owners")
	}

	truth, err := manager.MkTrue()
	must(err)
	queryRef, err := manager.NewRef(truth)
	must(err)
	originalQuery := queryRef
	owner := &clpRefOwner{manager: manager}
	owner.take(queryRef)
	refuse = owner.dispose
	failure = trigger()
	if !errors.Is(failure, ast.ErrASTPending) || len(owner.refs) != 1 || owner.refs[0] != originalQuery {
		t.Fatal("refused query cleanup lost its owner", failure)
	}
	if _, err := manager.RefGet(owner.refs[0]); err != nil {
		t.Fatal("refused query cleanup invalidated owner", err)
	}
	cleanup = nil
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
		t.Fatal("query cleanup is not retryable", failure)
	}
	must(cleanup.Cleanup())
	if owner.refs != nil || cleanup.CleanupCause() != nil {
		t.Fatal("query cleanup retry did not retire owner")
	}
}
