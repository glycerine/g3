package smtcore

import (
	"fmt"
	"runtime"
	"testing"
	"unsafe"
)

func BenchmarkSMTWatchListStorage(b *testing.B) {
	const workers, variables = 128, 1024
	for _, density := range []int{0, 1, 8, 32} {
		b.Run(fmt.Sprintf("OccupiedPer32_%d", density), func(b *testing.B) {
			if b.N != 1 {
				b.Skip("measure retained heap with -benchtime=1x")
			}
			// These are whole per-solver directories with two watches per variable.
			owners := make([][]SMTWatchList, workers)
			clause := new(SMTClause)
			before := scopeBenchmarkLiveHeap()
			b.ResetTimer()
			for i := range owners {
				a := make([]SMTWatchList, 2*variables)
				for j := range a {
					if j%32 >= density {
						continue
					}
					if j%2 == 0 {
						if err := a[j].InsertClause(clause); err != nil {
							b.Fatal(err)
						}
					} else {
						if err := a[j].InsertLiteral(SMTLiteral(j)); err != nil {
							b.Fatal(err)
						}
					}
				}
				owners[i] = a
			}
			b.StopTimer()
			after := scopeBenchmarkLiveHeap()
			b.ReportMetric(float64(after-before)/workers, "live-B/solver")
			b.ReportMetric(float64(unsafe.Sizeof(SMTWatchList{})), "B/header")
			runtime.KeepAlive(owners)
		})
	}
}
