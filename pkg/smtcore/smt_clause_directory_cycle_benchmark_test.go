package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Register and retire concrete clauses in a user scope. The solver and its
// Boolean variables survive each iteration, as in incremental verification.
func BenchmarkSMTConcreteClauseScope(b *testing.B) {
	const clauses = 256
	for _, width := range []int{3, 8, 65} {
		for _, mode := range []string{"Plain", "Proof", "Snapshot"} {
			b.Run(fmt.Sprintf("%d/%s", width, mode), func(b *testing.B) {
				m := astTestManager(b)
				if mode == "Proof" {
					astTestMust(b, m.ToggleProofMode(ast.ASTProofEnabled))
				}
				_, _, s := smtInternalizerFixture(b, m, nil)
				var atoms [160]ast.ASTHandle
				for i := range atoms {
					atoms[i] = smtRelevancyBool(b, m, fmt.Sprintf("cycle-%d", i))
				}
				literals := scopeRestorationLiterals(b, s, atoms[:]...)
				var proof ast.ASTHandle
				if mode == "Proof" {
					var err error
					proof, err = m.MkProofAsserted(atoms[0])
					astTestMust(b, err)
					astTestPin(b, m, proof)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for n := 0; n < b.N; n++ {
					if err := s.PushScope(); err != nil {
						b.Fatal(err)
					}
					s.pushBaseScope()
					for i := 0; i < clauses; i++ {
						start := i % (len(literals) - width + 1)
						if err := s.addClause(literals[start:start+width], i%2 == 0, proof); err != nil {
							b.Fatal(err)
						}
					}
					if len(s.auxClauses) != clauses {
						b.Fatal("missing installed clauses")
					}
					if mode == "Snapshot" {
						snapshot := s.Clauses()
						if len(snapshot) != clauses || len(snapshot[clauses-1].Literals) != width || !snapshot[0].Root {
							b.Fatal("incomplete public snapshot")
						}
					}
					if err := s.PopScopes(1); err != nil {
						b.Fatal(err)
					}
					if s.numClauseRecords() != 0 {
						b.Fatal("retired clauses remained in directory")
					}
				}
				b.StopTimer()
				b.ReportMetric(clauses, "clauses/op")
				astTestMust(b, s.Close())
			})
		}
	}
}
