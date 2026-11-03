package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyPolarityFinalRewriteSurvivesNativeCleanupRefusal(t *testing.T) {
	var manager *ast.ASTManager
	var final, intermediate ast.ASTRef
	var pending *legacyPolarityRewriteCleanupFailure
	active, called := false, false
	fixture := newFMFixtureTransport(t, false, func(f memory.Failure) error {
		if active && !called {
			called = true
			cause := finishLegacyPolarityRewrite(manager, final, intermediate, nil)
			if !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("polarity finalization lost its local owners: %v", cause)
			}
			if pending.final != final || pending.intermediate != intermediate || pending.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
				t.Fatal("polarity receipt changed concrete owner identity")
			}
			if cleanup := pending.Cleanup(); !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatalf("guarded polarity retry changed refusal: %v", cleanup)
			}
		}
		return f
	})
	manager = fixture.r.manager
	x := fixture.variable(0, fixture.integer)
	zero := fixture.number("0")
	intermediateRoot := fixture.own(manager.MkEq(x, zero))
	finalRoot := fixture.own(manager.MkNot(intermediateRoot))
	var err error
	intermediate, err = manager.NewRef(intermediateRoot)
	fmMust(t, err)
	final, err = manager.NewRef(finalRoot)
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
		t.Fatal("native numerical callback did not reach polarity finalization")
	}
	fmMust(t, manager.CleanupArithmeticNumeralInvocation(&invocation))

	for reference, want := range map[ast.ASTRef]ast.ASTHandle{intermediate: intermediateRoot, final: finalRoot} {
		got, getErr := manager.RefGet(reference)
		fmMust(t, getErr)
		if got != want {
			t.Fatal("refused finalization changed polarity root", got, want)
		}
	}
	stable := pending.Error()
	fmMust(t, pending.Cleanup())
	fmMust(t, pending.Cleanup())
	if pending.CleanupCause() != nil || pending.intermediate != (ast.ASTRef{}) || pending.final != (ast.ASTRef{}) || pending.Error() != stable || !errors.Is(pending.Unwrap(), ast.ErrASTPending) {
		t.Fatal("completed polarity cleanup lost stable receipt state")
	}
}
