package smtcore

import (
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Each shape creates the same number of binary applications. Vary the
// number of parent occurrences independently of the width and node arity.
func BenchmarkSMTEnodeParentShapes(b *testing.B) {
	const workers, width = 8, 512
	for _, shape := range []string{"star", "chain", "fork", "mixed"} {
		b.Run(shape, func(b *testing.B) {
			type worker struct {
				*smtEqualityUndoFixture
				terms   []ast.ASTHandle
				parents map[ast.ASTHandle]uint32
			}
			ws := make([]worker, workers)
			for i := range ws {
				f := newSMTEqualityUndoFixture(b, "parent-shape", width+1)
				w := &ws[i]
				w.smtEqualityUndoFixture = f
				w.parents = make(map[ast.ASTHandle]uint32)
				for k := range width {
					left, right := f.base[0], f.base[k+1]
					switch shape {
					case "chain":
						if k > 0 {
							left = w.terms[k-1]
						}
					case "fork":
						if k > 0 {
							left = w.terms[(k-1)/2]
						}
					case "mixed":
						if k%4 == 0 {
							right = left
						} else if k%4 == 1 && k > 1 {
							left = w.terms[k-1]
						}
						if k > 0 && k%4 == 0 {
							left, right = w.terms[k/4-1], w.terms[k/4-1]
						}
					}
					t, err := f.m.MkApp(f.binary, []ast.ASTHandle{left, right})
					astTestMust(b, err)
					astTestPin(b, f.m, t)
					w.terms = append(w.terms, t)
					w.parents[left]++
					w.parents[right]++
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
					astTestMust(b, w.s.PushScope())
					for _, t := range w.terms {
						astTestMust(b, w.s.Internalize(t, false))
					}
				}
				b.StopTimer()
				peak += scopeBenchmarkLiveHeap() - base
				for _, w := range ws {
					for _, nodes := range [][]ast.ASTHandle{w.base, w.terms} {
						for _, h := range nodes {
							n := w.s.context.enodes.get(h)
							if n == nil || n.NumParents() != w.parents[h] {
								b.Fatalf("incorrect number of parent occurrences of %v, want %d", h, w.parents[h])
							}
						}
					}
				}
				b.StartTimer()
				for _, w := range ws {
					astTestMust(b, w.s.PopScopes(1))
				}
				b.StopTimer()
				idle += scopeBenchmarkLiveHeap() - base
				for _, w := range ws {
					for _, h := range w.base {
						if w.s.context.enodes.get(h).NumParents() != 0 {
							b.Fatal("parent survived pop")
						}
					}
					for _, h := range w.terms {
						if w.s.context.enodes.get(h) != nil {
							b.Fatal("scoped application survived pop")
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
