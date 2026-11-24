package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// These are two actual selections of premises from the ticket protocol.
// A private plan can establish a contradiction. A satisfying selection
// must leave the source able to construct its own concrete model.
func TestTicketPrivatePlans(t *testing.T) {
	for _, name := range []string{"satisfiable", "contradiction"} {
		t.Run(name, func(t *testing.T) {
			c, source := orderSolver(t, appleBenchmarkInput(t, "ticket_private_"+name))
			m, err := c.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(source.solver)
			astTestMust(t, err)
			plan := shortPremisePlan{source: roots, selected: roots}
			batch := source.BeginOrderProofBatch()
			t.Cleanup(func() { astTestMust(t, batch.Close()) })
			for i := 0; i < 2; i++ {
				proved, err := batch.state.checkProofPlan(m, &plan, 100_000)
				if err != nil || proved != (name == "contradiction") {
					t.Fatalf("private refutation returned %t, %v", proved, err)
				}
			}
			result, err := source.Check()
			astTestMust(t, err)
			if name == "contradiction" {
				if result != lbool.False {
					t.Fatal("source failed to refute selected premises", result)
				}
				return
			}
			if result != lbool.True {
				t.Fatal("source failed to find a model", result)
			}
			model, err := source.Model()
			astTestMust(t, err)
			if model == nil {
				t.Fatal("missing source model")
			}
			astTestMust(t, model.Close())
		})
	}
}

func BenchmarkTicketPrivatePlans(b *testing.B) {
	for _, name := range []string{"satisfiable", "contradiction"} {
		b.Run(name, func(b *testing.B) {
			c, source := orderSolver(b, appleBenchmarkInput(b, "ticket_private_"+name))
			m, err := c.Manager()
			astTestMust(b, err)
			roots, err := solver.Assertions(source.solver)
			astTestMust(b, err)
			plan := shortPremisePlan{source: roots, selected: roots}
			batch := source.BeginOrderProofBatch()
			b.Cleanup(func() { astTestMust(b, batch.Close()) })
			b.ReportAllocs()
			for b.Loop() {
				proved, err := batch.state.checkProofPlan(m, &plan, 100_000)
				if err != nil || proved != (name == "contradiction") {
					b.Fatalf("private refutation returned %t, %v", proved, err)
				}
			}
		})
	}
}
