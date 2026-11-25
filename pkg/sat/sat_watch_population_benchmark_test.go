package sat

import (
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// Populate independent watch stores. The measured heap includes backing
// arrays and per-list bookkeeping. Long lists amortize that bookkeeping.
func BenchmarkSATWatchPopulation(b *testing.B) {
	for _, mode := range []string{"Binary", "Mixed", "Reused", "Short"} {
		b.Run(mode, func(b *testing.B) {
			const workers = 8
			lists, n := 32, 256
			if mode == "Short" {
				lists, n = 2048, 4
			}
			space := func() int64 {
				runtime.GC()
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				return int64(m.HeapAlloc)
			}
			stores := make([]*satSolverWatchStore, workers)
			dirs := make([][]satSolverWatchListRef, workers)
			for i := range stores {
				s, err := newSATSolverWatchStore(arena.NewBudget(^uint64(0)))
				if err != nil {
					b.Fatal(err)
				}
				stores[i] = s
				dirs[i] = make([]satSolverWatchListRef, lists)
				for j := range dirs[i] {
					r, e := s.NewList()
					if e != nil {
						b.Fatal(e)
					}
					dirs[i][j] = r
				}
			}
			var expected uint64
			before := space()
			for i, s := range stores {
				for _, r := range dirs[i] {
					for j := 0; j < n; j++ {
						value := uint32(2*j + 3)
						var w satWatched
						var err error
						switch {
						case mode == "Binary" || j%4 < 2:
							w, err = newSATBinaryWatch(Literal(value), j%2 == 0)
						case j%4 == 2:
							w, err = newSATTernaryWatch(Literal(value), Literal(value+1))
						default:
							g := uint32(1)
							if mode == "Reused" {
								g = uint32(1 + j%7)
							}
							w, err = newSATClauseWatch(Literal(value), satClauseOffset{owner: uint64(i + 1), slot: value, generation: g, storageKind: 1})
						}
						if err != nil {
							b.Fatal(err)
						}
						if err = s.Push(r, w); err != nil {
							b.Fatal(err)
						}
						expected += uint64(w.val1 + w.val2)
						if w.Clause() {
							o, _ := w.ClauseOffset()
							expected += o.owner + uint64(o.generation)
						}
					}
				}
			}
			live := space() - before
			b.ReportAllocs()
			b.ResetTimer()
			var got uint64
			for k := 0; k < b.N; k++ {
				for i, s := range stores {
					for _, r := range dirs[i] {
						it, e := s.Iter(r)
						if e != nil {
							b.Fatal(e)
						}
						for {
							w, ok, e := it.Next()
							if e != nil {
								b.Fatal(e)
							}
							if !ok {
								break
							}
							got += uint64(w.val1 + w.val2)
							if w.Clause() {
								o, _ := w.ClauseOffset()
								got += o.owner + uint64(o.generation)
							}
						}
					}
				}
			}
			b.StopTimer()
			if got != expected*uint64(b.N) {
				b.Fatalf("watch sum %d, want %d", got, expected*uint64(b.N))
			}
			reads := workers * lists * n
			b.ReportMetric(float64(live)/float64(reads), "live-B/watch")
			b.ReportMetric(float64(reads), "reads/op")
			for _, s := range stores {
				if err := s.Close(); err != nil {
					b.Fatal(err)
				}
			}
			runtime.KeepAlive(dirs)
		})
	}
}
