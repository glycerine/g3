package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyArrayEqualitiesConstructionRollbackPreservesNativeRefusal(t *testing.T) {
	var manager *ast.ASTManager
	var owner *legacyArrayEqualities
	var pending *legacyArrayEqualitiesConstructionFailure
	active, called := false, false
	originalCause := errors.New("find array equalities traversal failed")
	fixture := newFMFixtureTransport(t, false, func(f memory.Failure) error {
		if active && !called {
			called = true
			cause := closeFailedLegacyArrayEqualities(owner, originalCause)
			if !errors.Is(cause, originalCause) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("array-equality rollback lost its owner: %v", cause)
			}
			if pending.owner != owner || pending.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
				t.Fatal("array-equality rollback changed concrete owner identity")
			}
			if cleanup := pending.Cleanup(); !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatalf("guarded rollback retry changed refusal: %v", cleanup)
			}
		}
		return f
	})
	manager = fixture.r.manager
	left := fixture.constant("array-equalities-left", fixture.boolean)
	right := fixture.constant("array-equalities-right", fixture.boolean)
	leftRef, err := manager.NewRef(left)
	fmMust(t, err)
	rightRef, err := manager.NewRef(right)
	fmMust(t, err)
	owner = &legacyArrayEqualities{
		manager: manager, equalities: []ast.ASTRef{leftRef, rightRef},
		stores: map[ast.ASTHandle]bool{left: true, right: true},
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
		t.Fatal("native numerical callback did not reach equality rollback")
	}
	fmMust(t, manager.CleanupArithmeticNumeralInvocation(&invocation))

	if owner.manager != manager || len(owner.equalities) != 2 || owner.equalities[0] != leftRef || owner.equalities[1] != rightRef || !owner.stores[left] || !owner.stores[right] {
		t.Fatal("refused rollback changed array-equality metadata")
	}
	for reference, want := range map[ast.ASTRef]ast.ASTHandle{leftRef: left, rightRef: right} {
		got, getErr := manager.RefGet(reference)
		fmMust(t, getErr)
		if got != want {
			t.Fatal("refused rollback changed equality root", got, want)
		}
	}
	stable := pending.Error()
	fmMust(t, pending.Cleanup())
	fmMust(t, pending.Cleanup())
	if pending.CleanupCause() != nil || pending.owner != nil || owner.manager != nil || pending.Error() != stable || !errors.Is(pending.Unwrap(), originalCause) || !errors.Is(pending.Unwrap(), ast.ErrASTPending) {
		t.Fatal("completed equality rollback lost stable receipt state")
	}
}
