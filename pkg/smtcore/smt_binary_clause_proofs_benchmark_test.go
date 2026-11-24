package smtcore

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func BenchmarkSMTBinaryClauseProofMemory(b *testing.B) {
	const workers, width = 8, 1024
	for _, density := range []string{"Absent", "Last", "OnePer32", "Half", "Every"} {
		b.Run(density, func(b *testing.B) {
			type worker struct {
				s        *SMTInternalizerState
				m        *ast.ASTManager
				literals [64]SMTLiteral
				proofs   [width]ast.ASTHandle
			}
			ws := make([]worker, workers)
			hasProof := func(j int) bool {
				switch density {
				case "Last":
					return j == width-1
				case "OnePer32":
					return j%32 == 31
				case "Half":
					return j%2 == 0
				case "Every":
					return true
				}
				return false
			}
			var bytes int64
			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()
			for rep := 0; rep < b.N; rep++ {
				for i := range ws {
					w := &ws[i]
					w.m = astTestManager(b)
					astTestMust(b, w.m.ToggleProofMode(ast.ASTProofEnabled))
					var atoms, variables [64]ast.ASTHandle
					for j := range atoms {
						a := smtRelevancyBool(b, w.m, fmt.Sprintf("binary-proof-%d", j))
						variables[j] = a
						if j >= 32 {
							var err error
							a, err = w.m.MkNot(a)
							astTestMust(b, err)
						}
						astTestPin(b, w.m, a)
						atoms[j] = a
					}
					// Keep identical input expressions and proof nodes in every density.
					for j := range width {
						term, err := w.m.MkOr(atoms[j%32], atoms[32+j/32])
						astTestMust(b, err)
						proof, err := w.m.MkProofAsserted(term)
						astTestMust(b, err)
						astTestPin(b, w.m, proof)
						w.proofs[j] = proof
					}
					// The compact path is configured with proof generation disabled. Supply
					// already-owned explicit proof handles to exercise the publication API.
					astTestMust(b, w.m.ToggleProofMode(ast.ASTProofDisabled))
					_, _, w.s = smtInternalizerFixture(b, w.m, nil)
					astTestMust(b, w.s.SetBinaryClauseOptimization(true))
					for j, a := range variables {
						astTestMust(b, w.s.Internalize(a, false))
						v, ok := w.s.BoolVar(a)
						if !ok {
							b.Fatal("missing binary variable")
						}
						w.literals[j] = NewSMTLiteral(v, j >= 32)
					}
				}
				base := scopeBenchmarkLiveHeap()
				b.StartTimer()
				for i := range ws {
					w := &ws[i]
					for j := range width {
						pair := [2]SMTLiteral{w.literals[j%32], w.literals[32+j/32]}
						var proof ast.ASTHandle
						if hasProof(j) {
							proof = w.proofs[j]
						}
						astTestMust(b, w.s.addClauseKind(pair[:], j%2 == 0, SMTClauseAux, proof))
					}
				}
				b.StopTimer()
				bytes += scopeBenchmarkLiveHeap() - base
				for i := range ws {
					w := &ws[i]
					records := w.s.Clauses()
					if len(records) != width {
						b.Fatal("incomplete binary directory", len(records))
					}
					for j, r := range records {
						proof := ast.ASTHandle{}
						if hasProof(j) {
							proof = w.proofs[j]
						}
						pair := r.Literals
						if !r.Binary || r.Root != (j%2 == 0) || r.Lemma || r.Proof != proof || len(pair) != 2 || !((pair[0] == w.literals[j%32] && pair[1] == w.literals[32+j/32]) || (pair[1] == w.literals[j%32] && pair[0] == w.literals[32+j/32])) {
							b.Fatalf("incorrect published binary at %d", j)
						}
					}
					old := records[0].Literals[0]
					records[0].Literals[0] = old.Negated()
					if w.s.Clauses()[0].Literals[0] != old {
						b.Fatal("public snapshot aliases solver literals")
					}
					astTestMust(b, w.s.CheckInvariant())
				}
				b.StartTimer()
				for i := range ws {
					astTestMust(b, ws[i].s.Close())
				}
				b.StopTimer()
				for i := range ws {
					ws[i] = worker{}
				}
			}
			b.ReportMetric(float64(bytes)/float64(b.N*workers*width), "live-B/clause")
			runtime.KeepAlive(ws)
		})
	}
}
