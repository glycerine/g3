package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// A refused array_project_eqs_util rollback keeps its references reachable
// through the receipt until a later cleanup releases them.
func TestLegacyArrayProjectEqsRollbackPreservesNativeRefusal(t *testing.T) {
	var manager *ast.ASTManager
	var owner *legacyArrayProjectEqs
	var refs *legacyArrayRefs
	var pending *legacyArrayProjectEqsCleanupFailure
	var pendingRefs *legacyArrayRefsCleanupFailure
	active, called := false, false
	originalCause := errors.New("array_project_eqs failed")
	fixture := newFMFixtureTransport(t, false, func(f memory.Failure) error {
		if active && !called {
			called = true
			cause := closeFailedLegacyArrayProjectEqs(owner, originalCause)
			if !errors.Is(cause, originalCause) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("array_project_eqs rollback lost its owner: %v", cause)
			}
			if pending.owner != owner || pending.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
				t.Fatal("array_project_eqs rollback changed concrete owner identity")
			}
			if cleanup := pending.Cleanup(); !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatalf("guarded array_project_eqs retry changed refusal: %v", cleanup)
			}
			cause = closeLegacyArrayRefs(refs, originalCause)
			if !errors.Is(cause, originalCause) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pendingRefs) {
				t.Fatalf("scope rollback lost its owner: %v", cause)
			}
		}
		return f
	})
	manager = fixture.r.manager
	p, err := newLegacyArrayProjection(manager, fixture.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, p.Close()) })
	owner, err = newLegacyArrayProjectEqs(p)
	fmMust(t, err)
	variable := fixture.constant("project-eqs-cleanup-variable", fixture.integer)
	equality := fixture.own(manager.MkEq(variable, variable))
	fmMust(t, owner.vRefs.pin(equality))
	fmMust(t, owner.auxRefs.pin(variable))
	fmMust(t, owner.trueSub.Insert(equality, equality))
	refs = &legacyArrayRefs{manager: manager}
	fmMust(t, refs.pin(variable))

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
	if body == nil || !called || pending == nil || pendingRefs == nil {
		t.Fatal("native numerical callback did not reach array_project_eqs rollback")
	}
	fmMust(t, manager.CleanupArithmeticNumeralInvocation(&invocation))

	if owner.m != manager || len(owner.vRefs.refs) != 1 || len(owner.auxRefs.refs) != 1 || owner.trueSub == nil || len(refs.refs) != 1 {
		t.Fatal("refused rollback changed array_project_eqs state")
	}
	stable := pending.Error()
	fmMust(t, pending.Cleanup())
	fmMust(t, pending.Cleanup())
	fmMust(t, pendingRefs.Cleanup())
	if pending.CleanupCause() != nil || pending.owner != nil || owner.m != nil || len(refs.refs) != 0 || pending.Error() != stable || !errors.Is(pending.Unwrap(), originalCause) || !errors.Is(pending.Unwrap(), ast.ErrASTPending) {
		t.Fatal("completed array_project_eqs rollback lost stable receipt state")
	}
}
