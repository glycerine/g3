package smtcore

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// The full internalizer owns concrete clauses and binary watches. Count the
// directory, clause objects and watch buffers while the same inputs are live.
func BenchmarkSMTClauseDirectoryMemory(b *testing.B) {
	const workers, perWorker = 8, 1024
	for _, mode := range []string{"Binary", "Auxiliary", "Learned", "Justified", "Mixed"} {
		b.Run(mode, func(b *testing.B) {
			type worker struct {
				s        *SMTInternalizerState
				m        *ast.ASTManager
				literals [96]SMTLiteral
				proof    smtClauseTestJustification
			}
			ws := make([]worker, workers)
			shape := func(index int) string {
				if mode != "Mixed" {
					return mode
				}
				switch index % 8 {
				case 0:
					return "Learned"
				case 1, 2:
					return "Justified"
				case 3, 4:
					return "Binary"
				default:
					return "Auxiliary"
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()
			var live int64
			for rep := 0; rep < b.N; rep++ {
				for i := range ws {
					w := &ws[i]
					w.m = astTestManager(b)
					_, _, w.s = smtInternalizerFixture(b, w.m, nil)
					astTestMust(b, w.s.SetBinaryClauseOptimization(true))
					for j := range w.literals {
						a := smtRelevancyBool(b, w.m, fmt.Sprintf("directory-%d", j))
						astTestMust(b, w.s.Internalize(a, false))
						variable, ok := w.s.BoolVar(a)
						if !ok {
							b.Fatal("missing Boolean variable")
						}
						w.literals[j] = NewSMTLiteral(variable, false)
					}
				}
				wantBinary, wantLearned := 0, 0
				for j := range perWorker {
					switch shape(j) {
					case "Binary":
						wantBinary++
					case "Learned":
						wantLearned++
					}
				}
				base := scopeBenchmarkLiveHeap()
				b.StartTimer()
				for i := range ws {
					w := &ws[i]
					for j := range perWorker {
						ls := [3]SMTLiteral{w.literals[j%32], w.literals[32+(j/32)%32].Negated(), w.literals[64+(j%32+j/32)%32]}
						kind, n := SMTClauseAux, 3
						var proof SMTJustification
						switch shape(j) {
						case "Binary":
							n = 2
						case "Justified":
							proof = &w.proof
						case "Learned":
							kind = SMTClauseLearned
						}
						err := w.s.addClauseKindWithJustificationAndDeleteHandler(ls[:n], false, kind, ast.ASTHandle{}, proof, nil)
						if err != nil {
							b.Fatal(err)
						}
					}
				}
				b.StopTimer()
				live += scopeBenchmarkLiveHeap() - base
				for i := range ws {
					w := &ws[i]
					if w.s.binaryClauses.count() != wantBinary || len(w.s.lemmas) != wantLearned || w.s.numClauseRecords() != perWorker {
						b.Fatalf("directory contains %d binary, %d learned, %d total", w.s.binaryClauses.count(), len(w.s.lemmas), w.s.numClauseRecords())
					}
					public := w.s.Clauses()
					if len(public) != perWorker {
						b.Fatal("incomplete public snapshot")
					}
					for _, r := range public {
						if len(r.Literals) != 3 && !(r.Binary && len(r.Literals) == 2) {
							b.Fatal("incorrect published clause width")
						}
					}
					astTestMust(b, w.s.CheckInvariant())
				}
				b.StartTimer()
				for i := range ws {
					if err := ws[i].s.Close(); err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()
				for i := range ws {
					ws[i] = worker{}
				}
			}
			b.ReportMetric(float64(live)/float64(b.N*workers*perWorker), "live-B/clause")
			runtime.KeepAlive(ws)
		})
	}
}
