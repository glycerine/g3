package smtcore

import (
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Measure live heap after the last scope using these equivalence classes is
// removed. Run once per process with -benchtime=1x.
func BenchmarkFingerprintWorkerMemory(b *testing.B) {
	const workers, width = 1024, 16
	type worker struct {
		manager    *ast.ASTManager
		quantifier ast.ASTHandle
		id         uint32
		terms      [2 * width]ast.ASTHandle
		set        SMTFingerprintSet
	}
	owners := make([]worker, workers)
	for i := range owners {
		w := &owners[i]
		w.manager = astTestManager(b)
		boolean, err := w.manager.MkBoolSort()
		benchmarkMust(b, err)
		bound, err := w.manager.MkVar(0, boolean)
		benchmarkMust(b, err)
		var sorts [width]ast.ASTHandle
		var names [width]symbol.Symbol
		for j := range sorts {
			sorts[j] = boolean
			names[j] = symbol.Numerical(uint32(j))
		}
		w.quantifier, err = w.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts[:], Names: names[:], Body: bound})
		benchmarkMust(b, err)
		w.id, err = w.manager.GetASTID(w.quantifier)
		benchmarkMust(b, err)
		for j := range w.terms {
			w.terms[j], err = w.manager.MkConst(symbol.Numerical(uint32(1000+j)), boolean)
			benchmarkMust(b, err)
		}
	}
	live := func() int64 {
		runtime.GC()
		var s runtime.MemStats
		runtime.ReadMemStats(&s)
		return int64(s.HeapAlloc)
	}
	base := live()
	for i := range owners {
		w := &owners[i]
		benchmarkMust(b, w.set.PushScope())
		var args [width]*SMTEnode
		for j := range args {
			var err error
			root, err := NewSMTEnode(w.manager, w.terms[j], nil, SMTEnodeOptions{CongruenceEnabled: true})
			benchmarkMust(b, err)
			args[j], err = NewSMTEnode(w.manager, w.terms[width+j], nil, SMTEnodeOptions{CongruenceEnabled: true})
			benchmarkMust(b, err)
			args[j].root = root
		}
		got, err := w.set.Insert(w.quantifier, w.id, args[:])
		if err != nil || got == nil {
			b.Fatal("insert width", width, err)
		}
		// A narrow miss in the same scope leaves most previous scratch slots
		// beyond the slice length.
		if found, err := w.set.Contains(w.quantifier, w.id, args[:1]); err != nil || found {
			b.Fatal("narrow lookup", found, err)
		}
	}
	populated := live()
	for i := range owners {
		benchmarkMust(b, owners[i].set.PopScopes(1))
	}
	popped := live()
	for i := range owners {
		owners[i].set.Reset()
	}
	reset := live()
	b.ReportMetric(workers, "workers")
	b.ReportMetric(width, "roots/worker")
	b.ReportMetric(float64(populated-base)/workers, "active-B/worker")
	b.ReportMetric(float64(popped-base)/workers, "popped-B/worker")
	b.ReportMetric(float64(reset-base)/workers, "reset-B/worker")
	runtime.KeepAlive(owners)
}
