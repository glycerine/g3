package ast

import (
	"fmt"
	"runtime"
	"testing"
)

// The nodes already exist at the baseline. Measure the first marks' live
// heap separately from the cost of a subsequent full mark/reset cycle.
func BenchmarkASTMarkPopulation(b *testing.B) {
	const count = 8192
	for _, stride := range []int{1, 64, 512} {
		for _, planes := range []int{1, 2} {
			b.Run(fmt.Sprintf("stride%d/planes%d", stride, planes), func(b *testing.B) {
				m := astTestManager(b)
				sort, e := m.MkBoolSort()
				astTestMust(b, e)
				var handles [count]ASTHandle
				for i := range handles {
					h, e := m.MkVar(uint32(i), sort)
					astTestMust(b, e)
					astTestMust(b, m.IncRef(h))
					handles[i] = h
				}
				visit := func(set bool) {
					for i := 0; i < count; i += stride {
						h := handles[i]
						if set {
							if err := m.MarkNode1(h, true, 11); err != nil {
								b.Fatal(err)
							}
							if planes == 2 {
								if err := m.MarkNode2(h, true, 23); err != nil {
									b.Fatal(err)
								}
							}
						} else {
							if err := m.ResetNodeMark1(h, 11); err != nil {
								b.Fatal(err)
							}
							if planes == 2 {
								if err := m.ResetNodeMark2(h, 23); err != nil {
									b.Fatal(err)
								}
							}
						}
					}
				}
				live := func() int64 {
					runtime.GC()
					var s runtime.MemStats
					runtime.ReadMemStats(&s)
					return int64(s.HeapAlloc)
				}
				before := live()
				visit(true)
				extra := live() - before
				for i, h := range handles {
					a, e := m.IsNodeMarked1(h, 11)
					astTestMust(b, e)
					c, e := m.IsNodeMarked2(h, 23)
					astTestMust(b, e)
					if a != (i%stride == 0) || c != (planes == 2 && a) {
						b.Fatalf("wrong marks at %d", i)
					}
				}
				visit(false)
				b.ReportAllocs()
				b.ResetTimer()
				for n := 0; n < b.N; n++ {
					visit(true)
					visit(false)
				}
				b.StopTimer()
				for _, h := range handles {
					astTestMust(b, m.DecRef(h))
				}
				marked := count / stride
				b.ReportMetric(float64(extra), "first-live-B")
				b.ReportMetric(float64(marked*planes), "marks/op")
			})
		}
	}
}
