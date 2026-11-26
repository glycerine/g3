package sat

import (
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func BenchmarkSATClauseMixedChurn(b *testing.B) {
	for _, shape := range []string{"mixed", "wide-tail"} {
		b.Run(shape, func(b *testing.B) {
			const workers, base, scoped = 24, 32, 2048
			stores := make([]*ClauseAllocator, workers)
			refs := make([]StoredClauseRef, workers*scoped)
			widths := make([]int, scoped)
			literals := make([]Literal, 576)
			for i := range literals {
				literals[i] = Literal(2 * i)
			}
			for i := range widths {
				widths[i] = 3 + i%13
				if shape == "wide-tail" && i%4 == 0 {
					widths[i] = 64 + i/4
				}
			}
			for i := range stores {
				budget := arena.NewBudget(1 << 28)
				store, err := NewSATClauseAllocator(satClauseAllocatorConfig{
					Storage: arena.SmallBytesConfig{Descriptors: arena.Config{Budget: budget}},
				})
				if err != nil {
					b.Fatal(err)
				}
				stores[i] = store
				b.Cleanup(func() {
					if err := store.Close(); err != nil || budget.Used() != 0 {
						b.Fatal(err, budget.Used())
					}
				})
				for range base {
					if _, err := store.MkClause(literals[:3], false); err != nil {
						b.Fatal(err)
					}
				}
			}
			heap := func() int64 {
				runtime.GC()
				runtime.GC()
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				return int64(m.HeapAlloc)
			}
			before := heap()
			b.ResetTimer()
			for j := range b.N {
				for i := range refs {
					var err error
					refs[i], err = stores[i/scoped].MkClause(literals[:widths[i%scoped]], false)
					if err != nil {
						b.Fatal(err)
					}
				}
				if j == b.N-1 {
					b.StopTimer()
					b.ReportMetric(float64(heap()-before)/workers, "peak-B/worker")
					b.StartTimer()
				}
				for i := len(refs) - 1; i >= 0; i-- {
					if err := stores[i/scoped].Delete(refs[i]); err != nil {
						b.Fatal(err)
					}
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(heap()-before)/workers, "warm-B/worker")
		})
	}
}
