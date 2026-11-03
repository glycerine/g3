package bmc

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestLinearCompileRuleOwnsPartialVarSubstitutionConstructor(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var nativeOwner *linearCompileRuleOwner
	var pending error
	callbacks, intact := 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		callbacks++
		if nativeOwner != nil && pending == nil {
			beforeRef, beforeResult := nativeOwner.refs[0], nativeOwner.result
			pending = finishLinearCompileRule(nativeOwner, nil)
			intact = errors.Is(pending, ast.ErrASTPending) && nativeOwner.sub != nil && nativeOwner.refs[0] == beforeRef && nativeOwner.result == beforeResult
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
		if used := budget.Used(); used != 0 {
			t.Fatal("linear compile constructor fixture leaked bytes", used)
		}
	}()
	baseline := budget.Used()
	owner := &linearCompileRuleOwner{manager: manager}
	budget.FailAfter(1)
	cause := owner.newSubstitution(false)
	budget.FailAfter(-1)
	if cause == nil || owner.sub == nil || !errors.Is(cause, arena.ErrInjectedFailure) {
		t.Fatal("constructor frontier did not return a partial substitution", owner.sub, cause)
	}
	if got := finishLinearCompileRule(owner, cause); !errors.Is(got, arena.ErrInjectedFailure) {
		t.Fatal("constructor cause was not preserved", got)
	}
	if owner.sub != nil {
		t.Fatal("partial substitution remained owned after cleanup")
	}
	if used := budget.Used(); used != baseline {
		t.Fatal("partial substitution cleanup changed live bytes", baseline, used)
	}

	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("linear-compile-native-cleanup")
	must(err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	head, err := manager.MkApp(declaration, nil)
	must(err)
	nativeOwner = &linearCompileRuleOwner{manager: manager}
	must(nativeOwner.newSubstitution(false))
	ref, err := manager.NewRef(head)
	must(err)
	nativeOwner.refs = []ast.ASTRef{ref}
	nativeOwner.result, err = manager.NewRef(head)
	must(err)
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("native compile-rule cleanup touched dependent owners", body, callbacks, intact)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(pending, &cleanup) {
		t.Fatal("native compile-rule cleanup lost retry receipt", pending)
	}
	reported, unwrapped := cleanup.Error(), cleanup.Unwrap()
	must(cleanup.Cleanup())
	if cleanup.Error() != reported || cleanup.Unwrap() != unwrapped || nativeOwner.sub != nil || nativeOwner.refs[0] != (ast.ASTRef{}) || nativeOwner.result != (ast.ASTRef{}) {
		t.Fatal("native compile-rule retry lost stable ownership")
	}
}
