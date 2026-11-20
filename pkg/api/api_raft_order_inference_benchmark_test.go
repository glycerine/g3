package api

import (
	"os"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/solver"
)

// Profile only translation of an already loaded, immutable concrete formula.
func BenchmarkRaftOrderInference(b *testing.B) {
	for _, name := range []string{"001", "274"} {
		b.Run(name, func(b *testing.B) {
			data, err := os.ReadFile("test_vectors/raft_order/" + name + ".smt2")
			if err != nil {
				b.Fatal(err)
			}
			context, err := NewSMTContext()
			if err != nil {
				b.Fatal(err)
			}
			defer context.Close()
			s, err := context.NewSimpleSolver()
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			if err = s.LoadSMT2String(string(data)); err != nil {
				b.Fatal(err)
			}
			m, err := context.Manager()
			if err != nil {
				b.Fatal(err)
			}
			assertions, err := solver.Assertions(s.solver)
			if err != nil {
				b.Fatal(err)
			}
			first, ok, err := order.Relax(m, assertions)
			if err != nil || !ok {
				b.Fatal(ok, err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				query, ok, err := order.Relax(m, assertions)
				if err != nil || !ok || query != first {
					b.Fatal("unstable relaxation", ok, err)
				}
			}
		})
	}
}
