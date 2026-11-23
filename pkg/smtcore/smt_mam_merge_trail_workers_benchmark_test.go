package smtcore

import (
	"runtime"
	"testing"
)

func BenchmarkSMTMAMMergeTrailWorkers(b *testing.B) {
	const workers, merges = 1024, 256
	type worker struct {
		matcher     smtMAMImpl
		source, dst SMTEnode
	}
	liveBytes := func() int64 {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return int64(stats.HeapAlloc)
	}
	for _, labelled := range []bool{false, true} {
		name := "EmptyLabels"
		if labelled {
			name = "RepeatedLabels"
		}
		b.Run(name, func(b *testing.B) {
			var active, popped, reset int64
			for i := 0; i < b.N; i++ {
				group := make([]worker, workers)
				if labelled {
					for j := range group {
						group[j].source.labels.Insert(1)
					}
				}
				before := liveBytes()
				for j := range group {
					w := &group[j]
					w.matcher.PushScope()
					for k := 0; k < merges; k++ {
						w.matcher.AddEqEh(&w.source, &w.dst)
					}
				}
				active += liveBytes() - before
				for j := range group {
					group[j].matcher.PopScope(1)
					if !group[j].dst.labels.Empty() {
						b.Fatal("label survived pop")
					}
				}
				popped += liveBytes() - before
				for j := range group {
					group[j].matcher.Reset()
				}
				reset += liveBytes() - before
				runtime.KeepAlive(group)
			}
			count := float64(b.N) * workers
			b.ReportMetric(float64(max(0, active))/count, "active-B/worker")
			b.ReportMetric(float64(max(0, popped))/count, "popped-B/worker")
			b.ReportMetric(float64(max(0, reset))/count, "reset-B/worker")
		})
	}
}
