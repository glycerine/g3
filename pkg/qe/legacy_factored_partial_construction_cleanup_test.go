package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyFactoredPartialConstructionRollbackPreservesNativeRefusal(t *testing.T) {
	var manager *ast.ASTManager
	var owner *legacyFactoredPartialEquality
	var pending *legacyFactoredPartialConstructionFailure
	active, called := false, false
	originalCause := errors.New("factor selects construction failed")
	fixture := newFMFixtureTransport(t, false, func(f memory.Failure) error {
		if active && !called {
			called = true
			cause := closeFailedLegacyFactoredPartial(owner, originalCause)
			if !errors.Is(cause, originalCause) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("factored-partial rollback lost its owner: %v", cause)
			}
			if pending.owner != owner || pending.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
				t.Fatal("factored-partial rollback changed concrete owner identity")
			}
			if cleanup := pending.Cleanup(); !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatalf("guarded factored-partial retry changed refusal: %v", cleanup)
			}
		}
		return f
	})
	manager = fixture.r.manager
	family, err := manager.RegisterArrayPlugin()
	fmMust(t, err)
	arraySort := fixture.own(manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{fixture.integer}), fixture.integer))
	lhs := fixture.constant("factored-cleanup-lhs", arraySort)
	rhs := fixture.constant("factored-cleanup-rhs", arraySort)
	index := fixture.constant("factored-cleanup-index", fixture.integer)
	partial, err := newLegacyPartialEquality(manager, family, lhs, rhs, []ast.ASTHandle{index})
	fmMust(t, err)
	variable := fixture.constant("factored-cleanup-variable", fixture.integer)
	equality, err := manager.MkEq(variable, index)
	fmMust(t, err)
	variableRef, err := manager.NewRef(variable)
	fmMust(t, err)
	equalityRef, err := manager.NewRef(equality)
	fmMust(t, err)
	owner = &legacyFactoredPartialEquality{
		manager: manager, partial: partial,
		variables: []ast.ASTRef{variableRef}, equalities: []ast.ASTRef{equalityRef},
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
		t.Fatal("native numerical callback did not reach factored-partial rollback")
	}
	fmMust(t, manager.CleanupArithmeticNumeralInvocation(&invocation))

	if owner.manager != manager || owner.partial != partial || partial.manager != manager || len(owner.variables) != 1 || len(owner.equalities) != 1 {
		t.Fatal("refused rollback changed factored-partial metadata")
	}
	for reference, want := range map[ast.ASTRef]ast.ASTHandle{variableRef: variable, equalityRef: equality, partial.lhs: lhs, partial.rhs: rhs, partial.indices[0]: index} {
		got, getErr := manager.RefGet(reference)
		fmMust(t, getErr)
		if got != want {
			t.Fatal("refused rollback changed factored-partial root", got, want)
		}
	}
	stable := pending.Error()
	fmMust(t, pending.Cleanup())
	fmMust(t, pending.Cleanup())
	if pending.CleanupCause() != nil || pending.owner != nil || owner.manager != nil || partial.manager != nil || pending.Error() != stable || !errors.Is(pending.Unwrap(), originalCause) || !errors.Is(pending.Unwrap(), ast.ErrASTPending) {
		t.Fatal("completed factored-partial rollback lost stable receipt state")
	}
}

func TestLegacyFactoredPartialCloseContinuesPastConsumedReferences(t *testing.T) {
	fixture := newFMFixture(t)
	manager := fixture.r.manager
	family, err := manager.RegisterArrayPlugin()
	fmMust(t, err)
	arraySort := fixture.own(manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{fixture.integer}), fixture.integer))
	lhs := fixture.constant("factored-consumed-lhs", arraySort)
	rhs := fixture.constant("factored-consumed-rhs", arraySort)
	index := fixture.constant("factored-consumed-index", fixture.integer)
	partial, err := newLegacyPartialEquality(manager, family, lhs, rhs, []ast.ASTHandle{index})
	fmMust(t, err)
	partial.indices = append(partial.indices, ast.ASTRef{})
	variable := fixture.constant("factored-consumed-variable", fixture.integer)
	equality := fixture.own(manager.MkEq(variable, index))
	variableRef, err := manager.NewRef(variable)
	fmMust(t, err)
	equalityRef, err := manager.NewRef(equality)
	fmMust(t, err)
	owner := &legacyFactoredPartialEquality{
		manager: manager, partial: partial,
		variables:  []ast.ASTRef{variableRef, ast.ASTRef{}},
		equalities: []ast.ASTRef{equalityRef, ast.ASTRef{}},
	}

	fmMust(t, owner.Close())
	if owner.manager != nil || owner.partial != nil || len(owner.variables) != 0 || len(owner.equalities) != 0 {
		t.Fatal("factored-partial owner did not advance past consumed references")
	}
	if partial.manager != nil || len(partial.indices) != 0 || partial.lhs != (ast.ASTRef{}) || partial.rhs != (ast.ASTRef{}) || partial.decl != (ast.ASTRef{}) {
		t.Fatal("nested partial equality did not advance past consumed index")
	}
}
