package smtcore

import (
	"runtime"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Compare the incremental heap footprint of complete, independent solver
// states. Keep the input DAG alive so that Pop releases the internalized
// applications while the next round can reuse exactly the same input.
func BenchmarkSMTCongruenceScopeMemory(b *testing.B) {
	const workers, roots = 8, 64
	type worker struct {
		f     *smtEqualityUndoFixture
		input []ast.ASTHandle
	}
	for _, width := range []int{32, 256, 2048} {
		b.Run(strconv.Itoa(width), func(b *testing.B) {
			ws := make([]worker, workers)
			for i := range ws {
				f := newSMTEqualityUndoFixture(b, "scope-cg", roots)
				ws[i].f = f
				for k := 0; k < width; k++ {
					h, err := f.m.MkApp(f.binary, []ast.ASTHandle{f.base[k%roots], f.base[k/roots]})
					astTestMust(b, err)
					astTestPin(b, f.m, h)
					ws[i].input = append(ws[i].input, h)
					if k%16 == 0 {
						astTestMust(b, f.s.Internalize(h, false))
					}
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()
			base := scopeBenchmarkLiveHeap()
			var live, idle int64
			for rep := 0; rep < b.N; rep++ {
				b.StartTimer()
				for _, w := range ws {
					astTestMust(b, w.f.s.PushScope())
					for k, h := range w.input {
						if k%16 != 0 {
							astTestMust(b, w.f.s.Internalize(h, false))
						}
					}
				}
				b.StopTimer()
				live += scopeBenchmarkLiveHeap() - base
				for _, w := range ws {
					table := w.f.s.congruence.tables[smtHandleKey(w.f.binary)]
					if table == nil || table.small.count != width {
						b.Fatal("missing peak congruence entries", width)
					}
				}
				b.StartTimer()
				for _, w := range ws {
					astTestMust(b, w.f.s.PopScopes(1))
				}
				b.StopTimer()
				idle += scopeBenchmarkLiveHeap() - base
				for _, w := range ws {
					table := w.f.s.congruence.tables[smtHandleKey(w.f.binary)]
					if table.small.count != width/16 {
						b.Fatal("surviving signatures", table.small.count)
					}
					for k, h := range w.input {
						found := w.f.s.context.enodes.get(h)
						if (found != nil) != (k%16 == 0) {
							b.Fatal("surviving enode", k)
						}
						if found != nil {
							indexed, _, err := w.f.s.congruence.Find(found)
							astTestMust(b, err)
							if indexed != found {
								b.Fatal("missing representative", k)
							}
						}
					}
				}
			}
			n := float64(b.N * workers)
			b.ReportMetric(float64(live)/n, "peak-B/solver")
			b.ReportMetric(float64(idle)/n, "idle-B/solver")
			runtime.KeepAlive(ws)
		})
	}
}
