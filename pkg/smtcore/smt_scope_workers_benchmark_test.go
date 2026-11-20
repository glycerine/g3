package smtcore

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/ast"
)

type scopeBenchmarkWorker struct {
	s           *SMTInternalizerState
	relevance   *SMTRelevancyPropagator
	expressions []ast.ASTHandle
	literals    []SMTLiteral
}

func newScopeBenchmarkWorker(b *testing.B, width int) *scopeBenchmarkWorker {
	m := astTestManager(b)
	w := &scopeBenchmarkWorker{expressions: make([]ast.ASTHandle, width), literals: make([]SMTLiteral, width)}
	roots := make([]ast.ASTRef, width)
	b.Cleanup(func() {
		for i := range roots {
			benchmarkMust(b, m.DelRef(&roots[i]))
		}
	})
	for i := range w.expressions {
		h := smtRelevancyBool(b, m, fmt.Sprintf("worker-restore-%d", i))
		ref, err := m.NewRef(h)
		benchmarkMust(b, err)
		roots[i], w.expressions[i] = ref, h
	}
	_, w.relevance, w.s = smtInternalizerFixture(b, m, nil)
	benchmarkMust(b, w.s.ensureDelayedActivityQueue())
	return w
}

func (w *scopeBenchmarkWorker) round() (time.Duration, error) {
	s := w.s
	if err := s.PushScope(); err != nil {
		return 0, err
	}
	s.pushBaseScope()
	if err := s.beginSearchPersistence(1, SMTParamsState{}); err != nil {
		return 0, err
	}
	if err := s.PushScope(); err != nil {
		return 0, err
	}
	for i, h := range w.expressions {
		if err := s.Internalize(h, true); err != nil {
			return 0, err
		}
		if err := w.relevance.MarkRelevant(h); err != nil {
			return 0, err
		}
		lit, err := s.Literal(h)
		if err != nil {
			return 0, err
		}
		w.literals[i] = lit
	}
	if err := w.relevance.Propagate(); err != nil {
		return 0, err
	}
	for i := 0; i < len(w.literals); i += 2 {
		if err := s.AddLearnedClauseWithProof([]SMTLiteral{w.literals[i], w.literals[i+1].Negated(), w.literals[(i+2)%len(w.literals)]}, ast.ASTHandle{}); err != nil {
			return 0, err
		}
	}
	start := time.Now()
	err := s.PopScopes(1)
	elapsed := time.Since(start)
	if err != nil {
		return 0, err
	}
	if s.scopeLevel != 1 || len(s.lemmas) != len(w.literals)/2 || s.inconsistent {
		return 0, fmt.Errorf("invalid restored scope %d with %d lemmas", s.scopeLevel, len(s.lemmas))
	}
	if err = s.endSearchPersistence(); err != nil {
		return 0, err
	}
	if err = s.PopScopes(1); err != nil {
		return 0, err
	}
	if s.numClauseRecords() != 0 || len(s.booleanStack) != 0 {
		return 0, fmt.Errorf("outer scope retained clauses or variables")
	}
	return elapsed, nil
}

func scopeBenchmarkLiveHeap() int64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}

// ns/op measures aggregate throughput. pop-ns/op is mean wall time inside
// each solver's pop, including scheduling. idle-B/solver is the process-wide
// live heap growth from the unused fixtures to the emptied, warmed solvers.
// Use a fresh process with -benchtime=1x for fixture-B/solver. This includes
// the manager, rooted expressions and test fixture before benchmark calibration.
func BenchmarkSMTScopeRestorationWorkers(b *testing.B) {
	for _, count := range []int{1, 8, 48} {
		b.Run(fmt.Sprintf("Workers%d", count), func(b *testing.B) {
			benchmarkSMTScopedWorkers(b, count, smtHotPathWidth)
		})
	}
}

// Measure retained heap after a large scope and after running an oversubscribed worker set.
func BenchmarkSMTScopeRestorationPeakIdle(b *testing.B) {
	b.Run("Workers48Atoms2048", func(b *testing.B) { benchmarkSMTScopedWorkers(b, 48, 2048) })
	b.Run("Workers1024Atoms128", func(b *testing.B) { benchmarkSMTScopedWorkers(b, 1024, smtHotPathWidth) })
}

func benchmarkSMTScopedWorkers(b *testing.B, count, width int) {
	beforeFixtures := scopeBenchmarkLiveHeap()
	workers := make([]*scopeBenchmarkWorker, count)
	for i := range workers {
		workers[i] = newScopeBenchmarkWorker(b, width)
	}
	before := scopeBenchmarkLiveHeap()
	for _, w := range workers {
		_, err := w.round()
		benchmarkMust(b, err)
	}
	type result struct {
		pop time.Duration
		err error
	}
	results := make([]result, count)
	var wg sync.WaitGroup
	b.ReportAllocs()
	b.ResetTimer()
	for i, w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var sum time.Duration
			for j := i; j < b.N; j += count {
				d, err := w.round()
				if err != nil {
					results[i].err = err
					return
				}
				sum += d
			}
			results[i].pop = sum
		}()
	}
	wg.Wait()
	b.StopTimer()
	var sum time.Duration
	for _, r := range results {
		benchmarkMust(b, r.err)
		sum += r.pop
	}
	b.ReportMetric(float64(sum.Nanoseconds())/float64(b.N), "pop-ns/op")
	after := scopeBenchmarkLiveHeap()
	b.ReportMetric(float64(after-before)/float64(count), "idle-B/solver")
	if b.N == 1 {
		b.ReportMetric(float64(after-beforeFixtures)/float64(count), "fixture-B/solver")
	}
	runtime.KeepAlive(workers)
}
