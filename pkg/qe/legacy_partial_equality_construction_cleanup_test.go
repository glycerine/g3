package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyPartialEqualityConstructionRollbackPreservesNativeRefusal(t *testing.T) {
	var manager *ast.ASTManager
	var owner *legacyPartialEquality
	var pending *legacyPartialEqualityConstructionFailure
	active, called := false, false
	originalCause := errors.New("partial equality construction failed")
	fixture := newFMFixtureTransport(t, false, func(f memory.Failure) error {
		if active && !called {
			called = true
			cause := closeFailedLegacyPartialEquality(owner, originalCause)
			if !errors.Is(cause, originalCause) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("partial-equality rollback lost its owner: %v", cause)
			}
			if pending.owner != owner || pending.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
				t.Fatal("partial-equality rollback changed concrete owner identity")
			}
			if cleanup := pending.Cleanup(); !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatalf("guarded partial-equality retry changed refusal: %v", cleanup)
			}
		}
		return f
	})
	manager = fixture.r.manager
	family, err := manager.RegisterArrayPlugin()
	fmMust(t, err)
	arraySort := fixture.own(manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{fixture.integer}), fixture.integer))
	lhs := fixture.constant("partial-cleanup-lhs", arraySort)
	rhs := fixture.constant("partial-cleanup-rhs", arraySort)
	index0 := fixture.constant("partial-cleanup-index-0", fixture.integer)
	index1 := fixture.constant("partial-cleanup-index-1", fixture.integer)
	owner, err = newLegacyPartialEquality(manager, family, lhs, rhs, []ast.ASTHandle{index0, index1})
	fmMust(t, err)

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
		t.Fatal("native numerical callback did not reach partial-equality rollback")
	}
	fmMust(t, manager.CleanupArithmeticNumeralInvocation(&invocation))

	if owner.manager != manager || owner.family != family || len(owner.indices) != 2 {
		t.Fatal("refused rollback changed partial-equality metadata")
	}
	for reference, want := range map[ast.ASTRef]ast.ASTHandle{
		owner.lhs: lhs, owner.rhs: rhs, owner.indices[0]: index0, owner.indices[1]: index1,
	} {
		got, getErr := manager.RefGet(reference)
		fmMust(t, getErr)
		if got != want {
			t.Fatal("refused rollback changed partial-equality root", got, want)
		}
	}
	decl, err := manager.RefGet(owner.decl)
	fmMust(t, err)
	if decl.IsNull() {
		t.Fatal("refused rollback lost partial-equality declaration")
	}
	stable := pending.Error()
	fmMust(t, pending.Cleanup())
	fmMust(t, pending.Cleanup())
	if pending.CleanupCause() != nil || pending.owner != nil || owner.manager != nil || pending.Error() != stable || !errors.Is(pending.Unwrap(), originalCause) || !errors.Is(pending.Unwrap(), ast.ErrASTPending) {
		t.Fatal("completed partial-equality rollback lost stable receipt state")
	}
}
