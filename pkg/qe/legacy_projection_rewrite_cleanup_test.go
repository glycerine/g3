package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyProjectionRewriteFinalReferenceSurvivesNativeCleanupRefusal(t *testing.T) {
	var manager *ast.ASTManager
	var rewritten ast.ASTRef
	var pending *legacyProjectionRewriteCleanupFailure
	active, called := false, false
	fixture := newFMFixtureTransport(t, false, func(f memory.Failure) error {
		if active && !called {
			called = true
			cause := closeLegacyProjectionRewriteResult(manager, rewritten, nil)
			if !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("rewrite cleanup did not preserve its final reference: %v", cause)
			}
			if pending.reference != rewritten || pending.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
				t.Fatal("rewrite cleanup receipt lost its concrete owner")
			}
			if cleanup := pending.Cleanup(); !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatalf("guarded cleanup retry changed the refusal: %v", cleanup)
			}
		}
		return f
	})
	manager = fixture.r.manager
	projection, err := NewLegacyProjection(manager, fixture.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, projection.Close()) })

	x := fixture.variable(0, fixture.integer)
	zero := fixture.number("0")
	sum := fixture.own(manager.MkArithmeticAdd(x, zero))
	current, err := manager.NewRef(sum)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, manager.DelRef(&current)) })
	rewritten, err = projection.arithmetic.rewriteLegacy(sum)
	fmMust(t, err)
	rewrittenRoot, err := manager.RefGet(rewritten)
	fmMust(t, err)
	fmMust(t, manager.SetRef(current, rewrittenRoot))

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
		t.Fatal("native numerical callback did not reach final-reference cleanup")
	}
	fmMust(t, manager.CleanupArithmeticNumeralInvocation(&invocation))

	currentRoot, err := manager.RefGet(current)
	fmMust(t, err)
	if currentRoot != rewrittenRoot {
		t.Fatal("cleanup refusal changed the successfully rewritten result")
	}
	stable := pending.Error()
	fmMust(t, pending.Cleanup())
	fmMust(t, pending.Cleanup())
	if pending.CleanupCause() != nil || pending.Error() != stable || !errors.Is(pending.Unwrap(), ast.ErrASTPending) {
		t.Fatal("completed rewrite cleanup lost its stable originating failure")
	}
}
