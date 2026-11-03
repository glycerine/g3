package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyArraySelectConstructionRollbackPreservesNativeRefusal(t *testing.T) {
	var manager *ast.ASTManager
	var owner *legacyArraySelectProjection
	var pending *legacyArraySelectConstructionFailure
	active, called := false, false
	originalCause := errors.New("select projection construction failed")
	fixture := newFMFixtureTransport(t, false, func(f memory.Failure) error {
		if active && !called {
			called = true
			cause := closeFailedLegacyArraySelectProjection(owner, originalCause)
			if !errors.Is(cause, originalCause) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("select-projection rollback lost its owner: %v", cause)
			}
			if pending.owner != owner || pending.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
				t.Fatal("select-projection receipt changed concrete owner identity")
			}
			if cleanup := pending.Cleanup(); !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatalf("guarded select-projection retry changed refusal: %v", cleanup)
			}
		}
		return f
	})
	manager = fixture.r.manager
	formula := fixture.constant("select-cleanup-formula", fixture.boolean)
	first := fixture.constant("select-cleanup-first", fixture.integer)
	second := fixture.constant("select-cleanup-second", fixture.integer)
	formulaRef, err := manager.NewRef(formula)
	fmMust(t, err)
	firstRef, err := manager.NewRef(first)
	fmMust(t, err)
	secondRef, err := manager.NewRef(second)
	fmMust(t, err)
	owner = &legacyArraySelectProjection{
		manager: manager, formula: formulaRef,
		variables: []ast.ASTRef{firstRef, secondRef},
	}

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
		t.Fatal("native numerical callback did not reach select-projection rollback")
	}
	fmMust(t, manager.CleanupArithmeticNumeralInvocation(&invocation))

	if owner.manager != manager || owner.formula != formulaRef || len(owner.variables) != 2 || owner.variables[0] != firstRef || owner.variables[1] != secondRef {
		t.Fatal("refused rollback changed select-projection metadata")
	}
	for reference, want := range map[ast.ASTRef]ast.ASTHandle{formulaRef: formula, firstRef: first, secondRef: second} {
		got, getErr := manager.RefGet(reference)
		fmMust(t, getErr)
		if got != want {
			t.Fatal("refused rollback changed select-projection root", got, want)
		}
	}
	stable := pending.Error()
	fmMust(t, pending.Cleanup())
	fmMust(t, pending.Cleanup())
	if pending.CleanupCause() != nil || pending.owner != nil || owner.manager != nil || pending.Error() != stable || !errors.Is(pending.Unwrap(), originalCause) || !errors.Is(pending.Unwrap(), ast.ErrASTPending) {
		t.Fatal("completed select-projection cleanup lost stable receipt state")
	}
}

func TestLegacyArraySelectCloseContinuesPastConsumedVariable(t *testing.T) {
	fixture := newFMFixture(t)
	manager := fixture.r.manager
	formula := fixture.constant("select-consumed-formula", fixture.boolean)
	variable := fixture.constant("select-consumed-variable", fixture.integer)
	formulaRef, err := manager.NewRef(formula)
	fmMust(t, err)
	variableRef, err := manager.NewRef(variable)
	fmMust(t, err)
	owner := &legacyArraySelectProjection{
		manager: manager, formula: formulaRef,
		// The final cleared slot is the exact continuation state after DelRef
		// consumed a variable before transporting its error.
		variables: []ast.ASTRef{variableRef, ast.ASTRef{}},
	}
	fmMust(t, owner.Close())
	if owner.manager != nil || owner.formula != (ast.ASTRef{}) || len(owner.variables) != 0 {
		t.Fatal("select owner did not advance past consumed variable")
	}
}
