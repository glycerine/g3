package smtcore

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Measure storage over the live ASTs and enodes of 64 independent workers.
// Use -benchtime=1x in a fresh process when comparing live heap sizes.
func BenchmarkSMTFingerprintTupleMemory(b *testing.B) {
	const workers, tuples = 64, 256
	type worker struct {
		manager    *ast.ASTManager
		quantifier ast.ASTHandle
		id         uint32
		set        SMTFingerprintSet
		nodes      [tuples]*SMTEnode
		result     *SMTFingerprint
	}
	liveBytes := func() int64 {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return int64(stats.HeapAlloc)
	}
	run := func(b *testing.B, width int, resetWithCollisions bool) {
		owners := make([]worker, workers)
		for i := range owners {
			w := &owners[i]
			w.manager = astTestManager(b)
			m := w.manager
			boolean, err := m.MkBoolSort()
			benchmarkMust(b, err)
			bound, err := m.MkVar(0, boolean)
			benchmarkMust(b, err)
			sorts := make([]ast.ASTHandle, width)
			names := make([]symbol.Symbol, width)
			for j := range sorts {
				sorts[j], names[j] = boolean, symbol.Numerical(uint32(j))
			}
			w.quantifier, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: bound})
			benchmarkMust(b, err)
			w.id, err = m.GetASTID(w.quantifier)
			benchmarkMust(b, err)
			for j := range w.nodes {
				name := uint32(1000 + j)
				if resetWithCollisions {
					name = 1000 // Distinct enodes with one owner have identical tuple hashes.
				}
				owner, err := m.MkConst(symbol.Numerical(name), boolean)
				benchmarkMust(b, err)
				w.nodes[j], err = NewSMTEnode(m, owner, nil, SMTEnodeOptions{})
				benchmarkMust(b, err)
			}
		}
		var active, retired, released int64
		b.ResetTimer()
		b.StopTimer()
		for rep := 0; rep < b.N; rep++ {
			base := liveBytes()
			b.StartTimer()
			for i := range owners {
				w := &owners[i]
				if !resetWithCollisions {
					benchmarkMust(b, w.set.PushScope())
				}
				var args [8]*SMTEnode
				for j := 0; j < tuples; j++ {
					for k := 0; k < width; k++ {
						args[k] = w.nodes[(j+37*k)%tuples]
					}
					f, err := w.set.Insert(w.quantifier, w.id, args[:width])
					benchmarkMust(b, err)
					if f == nil {
						b.Fatal("distinct tuple treated as duplicate", j)
					}
					w.result = f
				}
			}
			b.StopTimer()
			active += liveBytes() - base
			for i := range owners {
				w := &owners[i]
				if resetWithCollisions {
					w.set.Reset()
				} else {
					benchmarkMust(b, w.set.PopScopes(1))
				}
			}
			retired += liveBytes() - base
			for i := range owners {
				w := &owners[i]
				arg, err := w.result.Arg(0)
				benchmarkMust(b, err)
				if arg != w.nodes[tuples-1] {
					b.Fatal("retiring a tuple changed its returned binding")
				}
				w.result = nil
				w.set.Reset()
			}
			released += liveBytes() - base
		}
		b.ReportMetric(float64(active)/float64(workers*tuples*b.N), "live-B/tuple")
		b.ReportMetric(float64(retired)/float64(workers*b.N), "one-result-B/worker")
		b.ReportMetric(float64(released)/float64(workers*b.N), "released-B/worker")
		runtime.KeepAlive(owners)
	}
	for _, width := range []int{1, 2, 3, 4, 5, 6, 8} {
		b.Run(fmt.Sprintf("Arity%d", width), func(b *testing.B) { run(b, width, false) })
	}
	b.Run("ResetWithCollisions", func(b *testing.B) { run(b, 1, true) })
}
