package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/solver"
)

// Parsing and source construction precede timing. Each iteration owns a new
// private arena, translates the assertions and solves the generated ground query.
func BenchmarkAppleWitnessInstances(b *testing.B) {
	for _, name := range []string{"cf_live-130", "cf_live-56"} {
		b.Run(name, func(b *testing.B) {
			input := appleBenchmarkInput(b, name)
			c, err := NewSMTContext()
			if err != nil {
				b.Fatal(err)
			}
			defer c.Close()
			s, err := c.NewSolver()
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			if err = s.LoadSMT2String(input); err != nil {
				b.Fatal(err)
			}
			assertions, err := solver.Assertions(s.solver)
			if err != nil {
				b.Fatal(err)
			}
			m, err := c.Manager()
			if err != nil {
				b.Fatal(err)
			}
			p := orderProofState{source: s}
			defer p.release()
			proofs := 0
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got, err := p.proveGoalInstances(m, assertions, orderProofSteps)
				if err != nil {
					b.Fatal(err)
				}
				if got {
					proofs++
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(proofs)/float64(b.N), "proofs/op")
			if name == "cf_live-130" && proofs != b.N {
				b.Fatalf("refuted %d of %d obligations", proofs, b.N)
			}
		})
	}
}
