package api

import (
	"os"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

func benchmarkOrderAssertions(b *testing.B, input string, create func(*SMTContext) (*APISimpleSolver, error)) (*ast.ASTManager, []ast.ASTHandle) {
	b.Helper()
	must := func(err error) {
		b.Helper()
		if err != nil {
			b.Fatal(err)
		}
	}
	c, err := NewSMTContext()
	must(err)
	b.Cleanup(func() { must(c.Close()) })
	s, err := create(c)
	must(err)
	b.Cleanup(func() { must(s.Close()) })
	must(s.LoadSMT2String(input))
	m, err := c.Manager()
	must(err)
	assertions, err := solver.Assertions(s.solver)
	must(err)
	return m, assertions
}

// Profile only translation of an already loaded, immutable concrete formula.
func BenchmarkRaftOrderInference(b *testing.B) {
	for _, name := range []string{"001", "274"} {
		b.Run(name, func(b *testing.B) {
			data, err := os.ReadFile("test_vectors/raft_order/" + name + ".smt2")
			if err != nil {
				b.Fatal(err)
			}
			m, assertions := benchmarkOrderAssertions(b, string(data), (*SMTContext).NewSimpleSolver)
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

// Time proof preparation alone. Obligation 001 has a direct instance, while
// 274 needs a solver. Both widths exercise the same source-level obligations.
func BenchmarkRaftProofPreparation(b *testing.B) {
	oneBit := oneBitRaftQueries(b)
	for _, tc := range []struct {
		name  string
		index int
	}{{"001", 0}, {"274", 273}} {
		mixed, err := os.ReadFile("test_vectors/raft_order/" + tc.name + ".smt2")
		if err != nil {
			b.Fatal(err)
		}
		for _, width := range []struct{ name, input string }{
			{"Mixed", string(mixed)}, {"OneBit", oneBit[tc.index]},
		} {
			b.Run(width.name+"/"+tc.name, func(b *testing.B) {
				m, assertions := benchmarkOrderAssertions(b, width.input, (*SMTContext).NewSolver)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					p, ok, err := order.ForProof(m, assertions, orderProofSteps)
					if err != nil || !ok || p.Refuted != (tc.index == 0) ||
						(p.Boolean != nil) != (tc.index == 273 && width.name == "OneBit") {
						b.Fatal("unexpected proof route", ok, p.Refuted, p.Boolean != nil, err)
					}
				}
			})
		}
	}
}
