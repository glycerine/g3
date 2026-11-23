package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Time the theory selection done by a fresh incremental push or an automatic
// check. The assertions share atoms, as premises do in a verifier batch.
func BenchmarkSMTAssertionFeatureSetup(b *testing.B) {
	for _, count := range []int{32, 256} {
		for _, mode := range []string{"FirstPush", "AutoCheck"} {
			b.Run(fmt.Sprintf("%s/%d", mode, count), func(b *testing.B) {
				m, cfg := smtModelGeneratorFixture(b)
				atoms := make([]ast.ASTHandle, count)
				clauses := make([]ast.ASTHandle, count)
				for i := range atoms {
					atoms[i] = smtRelevancyBool(b, m, fmt.Sprintf("setup-%d", i))
				}
				for i := range clauses {
					clause, err := m.MkOrArray([]ast.ASTHandle{atoms[i], atoms[(i+1)%count], atoms[(i+7)%count]})
					benchmarkMust(b, err)
					astTestPin(b, m, clause)
					clauses[i] = clause
				}
				config := SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg}
				b.ReportAllocs()
				b.ResetTimer()
				b.StopTimer()
				for i := 0; i < b.N; i++ {
					solver, err := NewSMTSolver(m, nil, "ALL", config)
					benchmarkMust(b, err)
					for _, clause := range clauses {
						benchmarkMust(b, solver.Assert(clause))
					}
					b.StartTimer()
					err = solver.kernel.setupAssertedBounds(mode == "AutoCheck")
					b.StopTimer()
					benchmarkMust(b, err)
					benchmarkMust(b, solver.Close())
				}
				b.ReportMetric(float64(count), "clauses/op")
			})
		}
	}
}
