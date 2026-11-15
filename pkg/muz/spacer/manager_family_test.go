package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
)

// AUDIT.md muz_spacer_01-F5: init_lemma_generalizers applies spacer.eq_prop
// (bound propagation off, auto-config simplex, no eq propagation, no eager
// eq axioms) only to m_pm.fparams(); fparams2/fparams3 keep the defaults.
func TestSpacerEqPropOnlyConfiguresFirstFamily(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	params, err := globals.Context.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := globals.Context.Del(&params); err != nil {
			t.Error(err)
		}
	}()
	name := "spacer.eq_prop"
	if err = globals.Context.SetBoolName(&params, &name, false); err != nil {
		t.Fatal(err)
	}
	owner, err := NewManager(manager, &params, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := owner.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	state := func(index int) smtcore.SMTArithState {
		t.Helper()
		solver, err := owner.contexts[index].Fresh()
		if err != nil {
			t.Fatal(err)
		}
		backend, ok := solver.factory.backend.(*smtcore.SMTSolver)
		if !ok {
			t.Fatal("virtual solver backend is not an SMT kernel")
		}
		result, err := backend.ArithmeticState()
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := state(0)
	if first.BoundProp != 0 || !first.AutoConfigSimplex || first.PropagateEqs || first.EagerEqAxioms {
		t.Fatalf("family 0 eq_prop settings: %+v", first)
	}
	for index := 1; index < 3; index++ {
		other := state(index)
		// smt_params defaults: BP_REFINE, no auto-config simplex,
		// propagate_eqs and eager_eq_axioms on.
		if other.BoundProp != 2 || other.AutoConfigSimplex || !other.PropagateEqs || !other.EagerEqAxioms {
			t.Fatalf("family %d took the eq_prop settings: %+v", index, other)
		}
	}
}
