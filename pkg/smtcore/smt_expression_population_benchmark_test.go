package smtcore

import (
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Each solver owns its table. The manager and existing handles stay alive at
// both collection boundaries. No enode storage is included in the increment.
func BenchmarkSMTExpressionTablePopulation(b *testing.B) {
	const workers, count = 8, 8192
	for _, mode := range []string{"Boolean", "Mixed", "Cached", "Sparse"} {
		b.Run(mode, func(b *testing.B) {
			m := astTestManager(b)
			sort, e := m.MkBoolSort()
			astTestMust(b, e)
			handles := make([]ast.ASTHandle, count)
			for i := range handles {
				h, e := m.MkVar(uint32(i), sort)
				astTestMust(b, e)
				astTestMust(b, m.IncRef(h))
				handles[i] = h
			}
			node := &SMTEnode{owner: handles[0]}
			tables := make([]*smtExprTable, workers)
			caches := make([]smtCacheGenerationTable, workers)
			start := scopeBenchmarkLiveHeap()
			stride := 1
			if mode == "Sparse" {
				stride = 16
			}
			for k := range tables {
				t := newSMTExprTable()
				tables[k] = t
				caches[k].t = t
				for i := 0; i < count; i += stride {
					h := handles[i]
					smtBoolVarTable{t}.set(h, int32(i))
					if mode != "Boolean" {
						smtEnodeTable{t}.set(h, node)
						smtRelevantTable{t}.set(h, i%2 == 0)
					}
					if mode == "Cached" {
						caches[k].set(h, uint32(i))
						smtAssignmentTable{t: t}.set(h, lbool.False)
					}
				}
			}
			retained := scopeBenchmarkLiveHeap() - start
			per := count / stride
			b.ReportAllocs()
			b.ResetTimer()
			var sum int64
			for r := 0; r < b.N; r++ {
				for k, t := range tables {
					for i := 0; i < count; i += stride {
						h := handles[i]
						v, ok := (smtBoolVarTable{t}).lookup(h)
						if !ok {
							b.Fatal("missing variable")
						}
						sum += int64(v)
						if mode != "Boolean" {
							if (smtEnodeTable{t}).get(h) != node {
								b.Fatal("wrong enode")
							}
							if (smtRelevantTable{t}).get(h) != (i%2 == 0) {
								b.Fatal("wrong relevance")
							}
						}
						if mode == "Cached" {
							g, ok := caches[k].get(h)
							if !ok || g != uint32(i) {
								b.Fatal("wrong cached generation")
							}
						}
					}
				}
			}
			b.StopTimer()
			want := int64(b.N) * workers * int64(per*(per-1)/2*stride)
			if sum != want {
				b.Fatalf("variable sum %d, want %d", sum, want)
			}
			b.ReportMetric(float64(retained)/float64(workers*per), "live-B/entry")
			b.ReportMetric(float64(workers*per), "reads/op")
			for _, h := range handles {
				astTestMust(b, m.DecRef(h))
			}
			runtime.KeepAlive(caches)
			runtime.KeepAlive(tables)
		})
	}
}
