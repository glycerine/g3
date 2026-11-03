package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyStoreEqualityFinalRewritePreservesResultOnNativeRefusal(t *testing.T) {
	var manager *ast.ASTManager
	var result *legacyArraySelectProjection
	var intermediate ast.ASTRef
	var pending *legacyStoreEqualityRewriteCleanupFailure
	active, called := false, false
	fixture := newFMFixtureTransport(t, false, func(f memory.Failure) error {
		if active && !called {
			called = true
			owner, cause := finishLegacyStoreEqualityRewrite(manager, result, intermediate, nil)
			if owner != nil {
				t.Fatal("refused store-equality finalization left caller ownership")
			}
			if !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("store-equality finalization lost its owners: %v", cause)
			}
			if pending.result != result || pending.intermediate != intermediate || pending.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
				t.Fatal("store-equality receipt changed concrete owner identity")
			}
			if cleanup := pending.Cleanup(); !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatalf("guarded store-equality retry changed refusal: %v", cleanup)
			}
		}
		return f
	})
	manager = fixture.r.manager
	formula := fixture.constant("store-cleanup-formula", fixture.boolean)
	selectVariable := fixture.constant("store-cleanup-select", fixture.integer)
	differenceVariable := fixture.constant("store-cleanup-difference", fixture.integer)
	intermediateRoot := fixture.own(manager.MkNot(formula))
	formulaRef, err := manager.NewRef(formula)
	fmMust(t, err)
	selectRef, err := manager.NewRef(selectVariable)
	fmMust(t, err)
	differenceRef, err := manager.NewRef(differenceVariable)
	fmMust(t, err)
	intermediate, err = manager.NewRef(intermediateRoot)
	fmMust(t, err)
	result = &legacyArraySelectProjection{
		manager: manager, formula: formulaRef,
		variables: []ast.ASTRef{selectRef, differenceRef},
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
		t.Fatal("native numerical callback did not reach store-equality finalization")
	}
	fmMust(t, manager.CleanupArithmeticNumeralInvocation(&invocation))

	if result.manager != manager || result.formula != formulaRef || len(result.variables) != 2 || result.variables[0] != selectRef || result.variables[1] != differenceRef {
		t.Fatal("refused finalization changed store-equality result")
	}
	for reference, want := range map[ast.ASTRef]ast.ASTHandle{
		formulaRef: formula, selectRef: selectVariable, differenceRef: differenceVariable, intermediate: intermediateRoot,
	} {
		got, getErr := manager.RefGet(reference)
		fmMust(t, getErr)
		if got != want {
			t.Fatal("refused finalization changed store-equality root", got, want)
		}
	}
	stable := pending.Error()
	fmMust(t, pending.Cleanup())
	fmMust(t, pending.Cleanup())
	if pending.CleanupCause() != nil || pending.result != nil || pending.intermediate != (ast.ASTRef{}) || result.manager != nil || pending.Error() != stable || !errors.Is(pending.Unwrap(), ast.ErrASTPending) {
		t.Fatal("completed store-equality cleanup lost stable receipt state")
	}
}

func TestLegacyStoreEqualityFinalRewriteTransfersClosedResultOnError(t *testing.T) {
	fixture := newFMFixture(t)
	manager := fixture.r.manager
	formula := fixture.constant("store-error-formula", fixture.boolean)
	variable := fixture.constant("store-error-variable", fixture.integer)
	intermediateRoot := fixture.own(manager.MkNot(formula))
	formulaRef, err := manager.NewRef(formula)
	fmMust(t, err)
	variableRef, err := manager.NewRef(variable)
	fmMust(t, err)
	intermediate, err := manager.NewRef(intermediateRoot)
	fmMust(t, err)
	result := &legacyArraySelectProjection{
		manager: manager, formula: formulaRef, variables: []ast.ASTRef{variableRef},
	}
	original := errors.New("store rewrite failed")
	owner, cause := finishLegacyStoreEqualityRewrite(manager, result, intermediate, original)
	if owner != nil {
		t.Fatal("failed store-equality finalization left caller ownership")
	}
	if !errors.Is(cause, original) || errors.Is(cause, ast.ErrASTHandle) {
		t.Fatal("store-equality finalization changed its rewrite error", cause)
	}
	if result.manager != nil {
		t.Fatal("store-equality finalization did not close its local owners")
	}
	if _, err := manager.RefGet(intermediate); err == nil {
		t.Fatal("store-equality finalization left its intermediate root live")
	}
}
