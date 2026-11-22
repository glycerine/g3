package api

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
)

var strategicWorkerHeap = flag.String("g3-strategic-worker-heap", "", "write a heap profile while the selected worker cohort is live")

// Each worker owns its context, arena and solver. Keep every solver alive
// after its first assertion. A fresh process and -benchtime=1x give the
// retained heap per initialized worker without calibration from earlier runs.
func BenchmarkStrategicSolverWorkers(b *testing.B) {
	type worker struct {
		context *SMTContext
		solver  *APISimpleSolver
	}
	liveHeap := func() uint64 {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return stats.HeapAlloc
	}
	for _, count := range []int{1, 16, 128, 1024} {
		b.Run(fmt.Sprintf("Workers%d", count), func(b *testing.B) {
			var retained int64
			for it := 0; it < b.N; it++ {
				b.StopTimer()
				before := liveHeap()
				workers := make([]worker, count)
				b.StartTimer()
				for i := range workers {
					ctx, err := NewSMTContext()
					if err != nil {
						b.Fatal(err)
					}
					m, err := ctx.Manager()
					if err != nil {
						b.Fatal(err)
					}
					truth, err := m.MkTrue()
					if err != nil {
						b.Fatal(err)
					}
					if err := m.IncRef(truth); err != nil {
						b.Fatal(err)
					}
					s, err := ctx.NewSolver()
					if err != nil {
						b.Fatal(err)
					}
					if err := s.Assert(truth); err != nil {
						b.Fatal(err)
					}
					if err := m.DecRef(truth); err != nil {
						b.Fatal(err)
					}
					workers[i] = worker{ctx, s}
				}
				b.StopTimer()
				retained += int64(liveHeap()) - int64(before)
				if *strategicWorkerHeap != "" && it == b.N-1 {
					runtime.GC()
					file, err := os.Create(*strategicWorkerHeap)
					if err != nil {
						b.Fatal(err)
					}
					if err := errors.Join(pprof.WriteHeapProfile(file), file.Close()); err != nil {
						b.Fatal(err)
					}
				}
				for i := len(workers) - 1; i >= 0; i-- {
					if err := workers[i].solver.Close(); err != nil {
						b.Fatal(err)
					}
					if err := workers[i].context.Close(); err != nil {
						b.Fatal(err)
					}
				}
			}
			b.ReportMetric(float64(retained)/float64(count)/float64(b.N), "live-B/worker")
		})
	}
}
