package smtcore

import (
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Compare the same number of survivors in one contiguous range or distributed
// throughout a large clause population. Use -benchtime=1x for live-heap runs.
func BenchmarkSMTSavedClauseAtomMemory(b *testing.B) {
	const workers, clauses, survivors = 32, 1024, 64
	type worker struct {
		manager     *ast.ASTManager
		expressions [4]ast.ASTHandle
		pool        smtClauseAtomPool
		live        [clauses]*SMTClause
	}
	for _, name := range []string{"Adjacent", "Spaced"} {
		b.Run(name, func(b *testing.B) {
			owners := make([]worker, workers)
			for i := range owners {
				w := &owners[i]
				w.manager = astTestManager(b)
				for j, n := range [...]string{"a", "b", "c"} {
					h := smtRelevancyBool(b, w.manager, n)
					astTestPin(b, w.manager, h)
					w.expressions[j+1] = h
				}
			}
			literals := []SMTLiteral{NewSMTLiteral(1, false), NewSMTLiteral(2, true), NewSMTLiteral(3, false)}
			var active, sparse, idle int64
			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()
			for rep := 0; rep < b.N; rep++ {
				base := scopeBenchmarkLiveHeap()
				b.StartTimer()
				for i := range owners {
					w := &owners[i]
					for j := range w.live {
						c, err := newSMTClause(w.manager, literals, SMTClauseLearned, nil, nil, true, w.expressions[:], &w.pool)
						benchmarkMust(b, err)
						w.live[j] = c
					}
				}
				b.StopTimer()
				active += scopeBenchmarkLiveHeap() - base
				for i := range owners {
					w := &owners[i]
					for j, c := range w.live {
						if name == "Adjacent" && j >= clauses-survivors || name == "Spaced" && j%(clauses/survivors) == 0 {
							continue
						}
						benchmarkMust(b, c.Close())
						w.live[j] = nil
					}
				}
				sparse += scopeBenchmarkLiveHeap() - base
				for i := range owners {
					w := &owners[i]
					for j, c := range w.live {
						if c == nil {
							continue
						}
						h, sign, err := c.Atom(1)
						if err != nil || h != w.expressions[2] || !sign {
							b.Fatal("survivor lost its signed atom", j, err)
						}
						benchmarkMust(b, c.Close())
						w.live[j] = nil
					}
				}
				idle += scopeBenchmarkLiveHeap() - base
				for i := range owners {
					owners[i].pool = smtClauseAtomPool{}
				}
			}
			b.ReportMetric(float64(active)/float64(b.N*workers*clauses), "live-B/clause")
			b.ReportMetric(float64(sparse)/float64(b.N*workers), "survivor-B/worker")
			b.ReportMetric(float64(idle)/float64(b.N*workers), "idle-B/worker")
			runtime.KeepAlive(owners)
		})
	}
}
