package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyWholeArrayFinalizationPreservesCurrentAndResultOnNativeRefusal(t *testing.T) {
	var manager *ast.ASTManager
	var current ast.ASTRef
	var result *legacyArrayProjectionResult
	var pending *legacyArrayFormulaCleanupFailure
	active, called := false, false
	fixture := newFMFixtureTransport(t, false, func(f memory.Failure) error {
		if active && !called {
			called = true
			cause := finishLegacyArrayFormula(manager, current, result, nil)
			if !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("whole-array finalization lost its owners: %v", cause)
			}
			if pending.current != current || pending.result != result || pending.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
				t.Fatal("whole-array finalization receipt changed owner identity")
			}
			if cleanup := pending.Cleanup(); !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatalf("guarded finalization retry changed refusal: %v", cleanup)
			}
		}
		return f
	})
	manager = fixture.r.manager
	formula := fixture.constant("whole-array-final-formula", fixture.boolean)
	variable := fixture.constant("whole-array-final-variable", fixture.integer)
	var err error
	current, err = manager.NewRef(formula)
	fmMust(t, err)
	formulaRef, err := manager.NewRef(formula)
	fmMust(t, err)
	variableRef, err := manager.NewRef(variable)
	fmMust(t, err)
	result = &legacyArrayProjectionResult{manager: manager, formula: formulaRef, variables: []ast.ASTRef{variableRef}}

	value, err := manager.NumeralService().NewValue()
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, manager.NumeralService().Del(&value)) })
	fmMust(t, manager.NumeralService().SetString(&value, "18446744073709551617/18446744073709551619"))
	runtime := manager.NumeralService().RuntimeOwner()
	fmMust(t, runtime.SetMaxCount(uint64(runtime.State().GlobalCount)+1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	fmMust(t, runtime.SetMaxCount(0))
	if body == nil || !called || pending == nil {
		t.Fatal("native numerical callback did not reach whole-array finalization")
	}
	fmMust(t, manager.CleanupArithmeticNumeralInvocation(&invocation))

	for reference, want := range map[ast.ASTRef]ast.ASTHandle{current: formula, formulaRef: formula, variableRef: variable} {
		got, getErr := manager.RefGet(reference)
		fmMust(t, getErr)
		if got != want {
			t.Fatal("whole-array owner changed after refusal", got, want)
		}
	}
	stable := pending.Error()
	fmMust(t, pending.Cleanup())
	fmMust(t, pending.Cleanup())
	if pending.CleanupCause() != nil || pending.Error() != stable || !errors.Is(pending.Unwrap(), ast.ErrASTPending) {
		t.Fatal("completed whole-array cleanup lost stable refusal")
	}
	if pending.current != (ast.ASTRef{}) || pending.result != nil || result.manager != nil {
		t.Fatal("whole-array cleanup retry left an owner live")
	}
}
