package smtcore

import (
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// At every density, give the same eight solvers the same applications and
// Boolean variables. Measure the incremental live heap with optional fields
// populated and again after all workers restore their scope.
func BenchmarkSMTEnodeFieldMemory(b *testing.B) {
	const workers, width = 8, 512
	for _, mode := range []string{"bare", "sparse", "equal", "disequal", "theory", "mixed"} {
		b.Run(mode, func(b *testing.B) {
			fieldForPair := func(pair int) string {
				switch mode {
				case "sparse":
					if pair%16 != 0 {
						return "bare"
					}
					fallthrough
				case "mixed":
					return []string{"equal", "disequal", "theory"}[pair%3]
				default:
					return mode
				}
			}
			type worker struct {
				f       *smtEqualityUndoFixture
				terms   []ast.ASTHandle
				reasons []SMTLiteral
			}
			ws := make([]worker, workers)
			for w := range ws {
				f := newSMTEqualityUndoFixture(b, "field-memory", 32)
				ws[w].f = f
				for k := 0; k < width; k++ {
					t, e := f.m.MkApp(f.binary, []ast.ASTHandle{f.base[k%32], f.base[k/32]})
					astTestMust(b, e)
					astTestPin(b, f.m, t)
					ws[w].terms = append(ws[w].terms, t)
				}
				for k := 0; k < width; k += 2 {
					ws[w].reasons = append(ws[w].reasons, smtDiseqLiteral(b, f.m, f.s, ws[w].terms[k], ws[w].terms[k+1]))
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()
			base := scopeBenchmarkLiveHeap()
			var peak, idle int64
			for rep := 0; rep < b.N; rep++ {
				b.StartTimer()
				for _, w := range ws {
					s := w.f.s
					astTestMust(b, s.PushScope())
					for _, t := range w.terms {
						astTestMust(b, s.Internalize(t, false))
					}
					for k := 0; k < width; k += 2 {
						left, right := s.context.enodes.get(w.terms[k]), s.context.enodes.get(w.terms[k+1])
						v := fieldForPair(k / 2)
						switch v {
						case "equal":
							astTestMust(b, s.addEquality(left, right, NewSMTEqualityEquation(w.reasons[k/2].Negated())))
						case "disequal":
							astTestMust(b, s.addDisequality(left, right, w.reasons[k/2]))
						case "theory":
							astTestMust(b, left.AddTheoryVar(3, SMTTheoryVar(k)))
							astTestMust(b, right.AddTheoryVar(3, SMTTheoryVar(k+1)))
						}
					}
				}
				b.StopTimer()
				peak += scopeBenchmarkLiveHeap() - base
				for _, w := range ws {
					for k := 0; k < width; k += 2 {
						a, c := w.f.s.context.enodes.get(w.terms[k]), w.f.s.context.enodes.get(w.terms[k+1])
						if a == nil || c == nil {
							b.Fatal("missing application")
						}
						field := fieldForPair(k / 2)
						if (a.root == c.root) != (field == "equal") {
							b.Fatalf("pair %d has incorrect equality", k/2)
						}
						entry, disequal := smtFindDisequality(a, c)
						if disequal != (field == "disequal") || disequal && entry.reason != w.reasons[k/2] {
							b.Fatalf("pair %d has incorrect disequality", k/2)
						}
						if field == "theory" {
							if a.TheoryVar(3) != SMTTheoryVar(k) || c.TheoryVar(3) != SMTTheoryVar(k+1) {
								b.Fatalf("pair %d lost theory bindings", k/2)
							}
						} else if a.TheoryVar(3) != SMTNullTheoryVar || c.TheoryVar(3) != SMTNullTheoryVar {
							b.Fatalf("unexpected theory binding on pair %d", k/2)
						}
						if field == "equal" {
							reasons, err := NewSMTAntecedentExpander().ExpandEquality(a, c)
							astTestMust(b, err)
							if len(reasons) != 1 || reasons[0] != w.reasons[k/2].Negated() {
								b.Fatalf("wrong reason for pair %d", k/2)
							}
						}
					}
				}
				b.StartTimer()
				for _, w := range ws {
					astTestMust(b, w.f.s.PopScopes(1))
				}
				b.StopTimer()
				idle += scopeBenchmarkLiveHeap() - base
				for _, w := range ws {
					for _, t := range w.terms {
						if w.f.s.context.enodes.get(t) != nil {
							b.Fatal("application survived its scope")
						}
					}
				}
			}
			div := float64(b.N * workers)
			b.ReportMetric(float64(peak)/div, "peak-B/solver")
			b.ReportMetric(float64(idle)/div, "idle-B/solver")
			runtime.KeepAlive(ws)
		})
	}
}
