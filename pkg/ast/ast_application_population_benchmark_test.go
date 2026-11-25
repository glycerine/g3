package ast

import (
	"fmt"
	"runtime"
	"testing"
	"unsafe"
)

// Measure the increase in live heap when four independent managers retain
// distinct applications. Input variables, declarations and root arrays are
// already live at the baseline. The native companion uses the same shapes.
func BenchmarkASTApplicationPopulation(b *testing.B) {
	const workers, terms = 4, 2048
	type worker struct {
		config        ASTManagerConfig
		manager       *ASTManager
		declaration   ASTHandle
		args          [8]ASTHandle
		inputs, roots [terms]ASTHandle
	}
	liveHeap := func() int64 {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return int64(stats.HeapAlloc)
	}
	for _, arity := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprint(arity), func(b *testing.B) {
			var ws [workers]worker
			for i := range ws {
				symbols, numerals, budget := astTestOwners(b)
				ws[i].config = ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget}
			}
			var live int64
			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()
			for n := 0; n < b.N; n++ {
				var oldNodes uint64
				for i := range ws {
					w := &ws[i]
					m, err := NewASTManager(w.config)
					astTestMust(b, err)
					w.manager = m
					sort, err := m.MkBoolSort()
					astTestMust(b, err)
					var domain [8]ASTHandle
					for j := range domain[:arity] {
						domain[j] = sort
					}
					w.declaration, err = m.MkFuncDecl(astTestSymbol(b, m, "population"), domain[:arity], sort, ASTFuncDeclInfo{})
					astTestMust(b, err)
					astTestMust(b, m.IncRef(w.declaration))
					for j := range w.inputs {
						w.inputs[j], err = m.MkVar(uint32(j), sort)
						astTestMust(b, err)
						astTestMust(b, m.IncRef(w.inputs[j]))
					}
					copy(w.args[:], w.inputs[:arity])
					oldNodes += uint64(m.liveNodes)
				}
				base := liveHeap()
				b.StartTimer()
				for i := range ws {
					w := &ws[i]
					for j := range w.inputs {
						w.args[0] = w.inputs[j]
						h, err := w.manager.MkApp(w.declaration, w.args[:arity])
						if err != nil {
							b.Fatal(err)
						}
						if err = w.manager.IncRef(h); err != nil {
							b.Fatal(err)
						}
						w.roots[j] = h
					}
				}
				b.StopTimer()
				live += liveHeap() - base
				var newNodes uint64
				for i := range ws {
					w := &ws[i]
					newNodes += uint64(w.manager.liveNodes)
					for j, h := range w.roots {
						w.args[0] = w.inputs[j]
						same, err := w.manager.MkApp(w.declaration, w.args[:arity])
						astTestMust(b, err)
						if same != h {
							b.Fatalf("application %d lost its canonical identity", j)
						}
						astTestMust(b, w.manager.DecRef(h))
					}
					for _, arg := range w.inputs {
						astTestMust(b, w.manager.DecRef(arg))
					}
					astTestMust(b, w.manager.DecRef(w.declaration))
					astTestMust(b, w.manager.Close())
				}
				if newNodes != oldNodes+workers*terms {
					b.Fatalf("created %d nodes, want %d", newNodes-oldNodes, workers*terms)
				}
			}
			b.ReportMetric(float64(live)/float64(b.N*workers*terms), "live-B/app")
			b.ReportMetric(float64(unsafe.Sizeof(astNodeSlot{})), "B/slot")
			b.ReportMetric(float64(unsafe.Sizeof(astNodePage{})), "B/page")
			b.ReportMetric(float64(unsafe.Sizeof(astTableEntry{})), "B/table-entry")
		})
	}
}
