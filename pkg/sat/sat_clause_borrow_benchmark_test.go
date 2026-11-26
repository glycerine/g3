package sat

import (
	"runtime"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// Populate one arena per solver. Keep the source locators alive at both
// snapshots, so the delta includes clause storage and its lookup directory.
func BenchmarkSATClauseBorrowPopulation(b *testing.B) {
	for _, width := range []int{3, 8, 65} {
		b.Run(strconv.Itoa(width), func(b *testing.B) {
			const workers, clauses = 24, 2048
			var allocators [workers]*ClauseAllocator
			refs := make([]StoredClauseRef, workers*clauses)
			literals := make([]Literal, width)
			for i := range literals {
				literals[i] = Literal(2 * i)
			}
			for i := range allocators {
				allocator, err := NewSATClauseAllocator(satClauseAllocatorConfig{
					Storage: arena.SmallBytesConfig{Descriptors: arena.Config{Budget: arena.NewBudget(^uint64(0))}},
				})
				if err != nil {
					b.Fatal(err)
				}
				allocators[i] = allocator
				b.Cleanup(func() {
					if err := allocator.Close(); err != nil {
						b.Fatal(err)
					}
				})
			}
			heap := func() int64 {
				runtime.GC()
				runtime.GC()
				var stats runtime.MemStats
				runtime.ReadMemStats(&stats)
				return int64(stats.HeapAlloc)
			}
			before := heap()
			for i := range refs {
				ref, err := allocators[i/clauses].MkClause(literals, false)
				if err != nil {
					b.Fatal(err)
				}
				refs[i] = ref
			}
			liveBytes := heap() - before
			b.ReportAllocs()
			b.ResetTimer()
			var count uint64
			for i := 0; i < b.N; i++ {
				for j, ref := range refs {
					view, err := allocators[j/clauses].view(ref)
					if err != nil {
						b.Fatal(err)
					}
					count += uint64(view.size())
				}
			}
			b.StopTimer()
			if count != uint64(b.N)*uint64(len(refs))*uint64(width) {
				b.Fatal("clause lookup lost a record")
			}
			b.ReportMetric(float64(liveBytes)/float64(len(refs)), "live-B/clause")
			b.ReportMetric(float64(len(refs)), "views/op")
		})
	}
}
