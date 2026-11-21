package ast

import (
	"runtime"
	"testing"
)

// Measure cache storage above the same rooted ASTs in 1,024 independent
// managers. Run once per fresh process with -benchtime=1x.
func BenchmarkASTActivityCacheWorkerMemory(b *testing.B) {
	const workers, width = 1024, 256
	type worker struct {
		manager *ASTManager
		keys    [width]ASTHandle
		cache   *ActivityCache
	}
	owners := make([]worker, workers)
	for i := range owners {
		w := &owners[i]
		w.manager = astTestManager(b)
		boolean, err := w.manager.MkBoolSort()
		astTestMust(b, err)
		for j := range w.keys {
			w.keys[j], err = w.manager.MkVar(uint32(j), boolean)
			astTestMust(b, err)
			astTestMust(b, w.manager.IncRef(w.keys[j]))
		}
	}
	live := func() int64 {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return int64(stats.HeapAlloc)
	}
	baseline := live()
	for i := range owners {
		w := &owners[i]
		var err error
		w.cache, err = NewActivityCache(w.manager, nil)
		astTestMust(b, err)
		for j, k := range w.keys {
			astTestMust(b, w.cache.Insert(k, w.keys[(j+1)%width]))
		}
	}
	populated := live()
	for i := range owners {
		astTestMust(b, owners[i].cache.Reset())
	}
	reset := live()
	b.ReportMetric(workers, "workers")
	b.ReportMetric(width, "entries/worker")
	b.ReportMetric(float64(populated-baseline)/workers, "active-B/worker")
	b.ReportMetric(float64(reset-baseline)/workers, "reset-B/worker")
	for i := range owners {
		w := &owners[i]
		astTestMust(b, w.cache.Close())
		for _, k := range w.keys {
			astTestMust(b, w.manager.DecRef(k))
		}
	}
	runtime.KeepAlive(owners)
}
