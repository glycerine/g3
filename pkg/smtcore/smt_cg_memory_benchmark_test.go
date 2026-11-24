package smtcore

import (
	"runtime"
	"strconv"
	"testing"
)

// Measure the table storage separately from the enodes it indexes. Each worker
// owns its roots and its tables. At widths up to 64, half the declarations are
// unary. The wider declarations are binary. Use -benchtime=1x for heap samples.
func BenchmarkSMTCongruenceTableMemory(b *testing.B) {
	const workers, declarations, roots = 32, 8, 64
	type worker struct {
		nodes  [roots]SMTEnode
		tables [declarations]smtCgSmallTable
	}
	key := func(w *worker, i int, unary bool) (first, second, value *SMTEnode) {
		first = &w.nodes[i%roots]
		if !unary {
			second = &w.nodes[i/roots]
		}
		return first, second, &w.nodes[(i+1)%roots]
	}
	for _, peak := range []int{1, 4, 32, 256, 2048} {
		b.Run(strconv.Itoa(peak), func(b *testing.B) {
			owners := make([]worker, workers)
			for i := range owners {
				m := astTestManager(b)
				for j := range owners[i].nodes {
					h := smtRelevancyBool(b, m, "cg"+strconv.Itoa(j))
					astTestPin(b, m, h)
					owners[i].nodes[j].owner = h
				}
			}
			var live, sparse int64
			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()
			for rep := 0; rep < b.N; rep++ {
				base := scopeBenchmarkLiveHeap()
				b.StartTimer()
				for i := range owners {
					w := &owners[i]
					for j := range w.tables {
						for k := range peak {
							left, right, value := key(w, k, peak <= roots && j%2 == 0)
							w.tables[j].set(left, right, value)
						}
					}
				}
				b.StopTimer()
				live += scopeBenchmarkLiveHeap() - base
				for i := range owners {
					w := &owners[i]
					for j := range w.tables {
						table := &w.tables[j]
						for k := range peak {
							left, right, value := key(w, k, peak <= roots && j%2 == 0)
							if table.get(left, right) != value {
								b.Fatal("missing entry", k)
							}
							if k%16 != 0 {
								table.del(left, right)
							}
						}
						if table.count != (peak+15)/16 {
							b.Fatal("incorrect survivor count", table.count)
						}
					}
				}
				sparse += scopeBenchmarkLiveHeap() - base
				for i := range owners {
					clear(owners[i].tables[:])
				}
			}
			n := float64(b.N * workers * declarations)
			b.ReportMetric(float64(live)/n/float64(peak), "live-B/entry")
			b.ReportMetric(float64(live)/n, "live-B/table")
			b.ReportMetric(float64(sparse)/n, "survivor-B/table")
			runtime.KeepAlive(owners)
		})
	}
}
