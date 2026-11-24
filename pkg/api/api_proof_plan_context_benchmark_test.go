package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/solver"
)

// Alternate an inconsistent pair and a satisfiable singleton to exercise
// context construction or reuse with a fresh private solver on every check.
func BenchmarkPrivateProofPlanContexts(b *testing.B) {
	ctx, source := orderSolver(b, "(declare-const a Bool)(assert a)(assert (not a))")
	m, err := ctx.Manager()
	astTestMust(b, err)
	forms, err := solver.Assertions(source.solver)
	astTestMust(b, err)
	plans := [...]shortPremisePlan{
		{source: forms, selected: forms},
		{source: forms[1:], selected: forms[1:]},
	}
	for _, mode := range []string{"Fresh", "Batch"} {
		b.Run(mode, func(b *testing.B) {
			batch := source.BeginOrderProofBatch()
			b.Cleanup(func() { astTestMust(b, batch.Close()) })
			if mode == "Fresh" {
				batch.state.plans.enabled = false
			}
			i := 0
			b.ReportAllocs()
			for b.Loop() {
				yes, err := batch.state.checkProofPlan(m, &plans[i%2], 8000)
				if err != nil || yes != (i%2 == 0) {
					b.Fatalf("plan %d returned %t, %v", i, yes, err)
				}
				i++
			}
		})
	}
}
