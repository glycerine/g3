package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyArrayRewriteConstructionRollbackPreservesNativeRefusal(t *testing.T) {
	var manager *ast.ASTManager
	var output ast.ASTRef
	var pending *legacyArrayRewriteConstructionFailure
	active, called := false, false
	originalCause := errors.New("array theory rewrite failed")
	fixture := newFMFixtureTransport(t, false, func(f memory.Failure) error {
		if active && !called {
			called = true
			cause := closeFailedLegacyArrayRewrite(manager, output, originalCause)
			if !errors.Is(cause, originalCause) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("array rewrite rollback lost its output: %v", cause)
			}
			if pending.result != output || pending.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
				t.Fatal("array rewrite rollback changed concrete output identity")
			}
			if cleanup := pending.Cleanup(); !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatalf("guarded rewrite retry changed refusal: %v", cleanup)
			}
		}
		return f
	})
	manager = fixture.r.manager
	x := fixture.variable(0, fixture.integer)
	zero := fixture.number("0")
	expression := fixture.own(manager.MkArithmeticAdd(x, zero))
	var err error
	output, err = manager.NewRef(expression)
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
		t.Fatal("native numerical callback did not reach array rewrite rollback")
	}
	fmMust(t, manager.CleanupArithmeticNumeralInvocation(&invocation))

	got, err := manager.RefGet(output)
	fmMust(t, err)
	if got != expression {
		t.Fatal("refused rewrite rollback changed output root", got, expression)
	}
	stable := pending.Error()
	fmMust(t, pending.Cleanup())
	fmMust(t, pending.Cleanup())
	if pending.CleanupCause() != nil || pending.result != (ast.ASTRef{}) || pending.Error() != stable || !errors.Is(pending.Unwrap(), originalCause) || !errors.Is(pending.Unwrap(), ast.ErrASTPending) {
		t.Fatal("completed array rewrite rollback lost stable receipt state")
	}
}
