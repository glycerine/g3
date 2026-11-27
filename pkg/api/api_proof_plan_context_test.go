package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestVerificationPlansStartWithFreshAssertions(t *testing.T) {
	// Both plans must reach the private solver to exercise its rotation.
	// The satisfiable subset requires two distinct domain elements.
	ctx, source := orderSolver(t, "(declare-sort U 0)(declare-const a U)(declare-const b U)(assert (= a b))(assert (not (= a b)))")
	m, err := ctx.Manager()
	astTestMust(t, err)
	forms, err := solver.Assertions(source.solver)
	astTestMust(t, err)
	if len(forms) != 2 {
		t.Fatal("expected two original assertions")
	}
	batch := source.BeginOrderProofBatch()
	defer func() { astTestMust(t, batch.Close()) }()
	var first *SMTContext
	for i := 0; i < maxPlansPerContext+2; i++ {
		selected := forms
		if i%2 != 0 {
			selected = forms[1:]
		}
		plan := &shortPremisePlan{source: selected, selected: selected}
		proved, err := batch.state.checkProofPlan(m, plan, 8000)
		astTestMust(t, err)
		if proved != (len(selected) == 2) {
			t.Fatalf("plan %d with %d assertions returned %t", i, len(selected), proved)
		}
		if len(m.ResourceLimit().Children) != 0 {
			t.Fatal("parent retained a cancellation registration after the check")
		}
		aux := batch.state.plans.aux
		if ast.ASTSourceDebug || diagnostic.TraceCompiled() {
			if aux != nil {
				t.Fatal("retained a private context under diagnostics")
			}
			continue
		}
		if aux == nil || aux.child.solver != nil {
			t.Fatal("expected a private context with its previous solver released")
		}
		private, err := aux.ctx.Manager()
		astTestMust(t, err)
		if private == m || private.ResourceLimit().RunSignals != nil || private.ResourceLimit().Limit != 0 {
			t.Fatal("private plan kept the parent's manager, signals or search limit")
		}
		switch {
		case i == 0:
			first = aux.ctx
		case i < maxPlansPerContext && first != aux.ctx:
			t.Fatal("rotated before the reuse limit")
		case i >= maxPlansPerContext && first == aux.ctx:
			t.Fatal("kept a context past the reuse limit")
		}
	}
	astTestMust(t, batch.Close())
	if batch.state.plans.aux != nil {
		t.Fatal("closing the batch retained its private context")
	}
	answer, err := source.Check()
	astTestMust(t, err)
	if answer != lbool.False {
		t.Fatal("private plan changed the source assertions")
	}
}

func TestVerificationPlansRecoverAfterInterruptedInput(t *testing.T) {
	ctx, source := orderSolver(t, "(declare-const a Bool)(assert a)(assert (not a))")
	m, err := ctx.Manager()
	astTestMust(t, err)
	forms, err := solver.Assertions(source.solver)
	astTestMust(t, err)
	batch := source.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	plan := &shortPremisePlan{source: forms, selected: forms}
	check := func(steps uint32, want bool) {
		t.Helper()
		yes, err := batch.state.checkProofPlan(m, plan, steps)
		if err != nil || yes != want || len(m.ResourceLimit().Children) != 0 {
			t.Fatalf("proof with %d steps returned %t, %v with %d registered children", steps, yes, err, len(m.ResourceLimit().Children))
		}
	}
	check(8000, true)
	check(1, false)
	check(8000, true)
	m.ResourceLimit().Cancel()
	check(8000, false)
	if batch.state.plans.aux != nil {
		t.Fatal("retained canceled context")
	}
	m.ResourceLimit().ResetCancel()
	check(8000, true)
	m.Budget().FailAfter(0)
	yes, err := batch.state.checkProofPlan(m, plan, 8000)
	m.Budget().FailAfter(-1)
	if err == nil || yes || batch.state.plans.aux != nil {
		t.Fatalf("injected allocation failure returned %t, %v, retained %t", yes, err, batch.state.plans.aux != nil)
	}
	check(8000, true)
}
