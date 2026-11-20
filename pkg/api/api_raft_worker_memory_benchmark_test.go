package api

import (
	"os"
	"runtime"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// Run with -cpu=1 in a fresh process. Heap metrics use a full GC before each
// observation. The timer covers the proof on each independent solver.
func BenchmarkRaftIdleSolverMemory(b *testing.B) {
	data, err := os.ReadFile("test_vectors/raft_order/001.smt2")
	if err != nil {
		b.Fatal(err)
	}
	input := string(data)
	for _, mode := range []string{"Concrete", "Order", "OrderBatch"} {
		for _, count := range []int{1, 8, 32, 64} {
			b.Run(mode+"/"+strconv.Itoa(count), func(b *testing.B) {
				b.StopTimer()
				var loaded, active, idle, budget float64
				for i := 0; i < b.N; i++ {
					l, working, h, a := benchRaftIdleGroup(b, input, count, mode)
					loaded += l
					active += working
					idle += h
					budget += a
				}
				n := float64(b.N) * float64(count)
				b.ReportMetric(loaded/n, "loaded-heap-B/solver")
				if mode == "OrderBatch" {
					b.ReportMetric(active/n, "active-heap-B/solver")
				}
				b.ReportMetric(idle/n, "idle-heap-B/solver")
				b.ReportMetric(budget/n, "idle-budget-B/solver")
			})
		}
	}
}

func benchRaftIdleGroup(b *testing.B, input string, count int, mode string) (loaded, active, idle, budget float64) {
	heap := func() float64 {
		runtime.GC()
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		return float64(mem.HeapAlloc)
	}
	base := heap()
	contexts := make([]*SMTContext, count)
	solvers := make([]*APISimpleSolver, count)
	var batches []*OrderProofBatch
	defer func() {
		for _, batch := range batches {
			if err := batch.Close(); err != nil {
				b.Error(err)
			}
		}
		for i, c := range contexts {
			if solvers[i] != nil {
				if err := solvers[i].Close(); err != nil {
					b.Error(err)
				}
			}
			if c != nil {
				if err := c.Close(); err != nil {
					b.Error(err)
				}
			}
		}
	}()
	for i := range solvers {
		var err error
		contexts[i], err = NewSMTContext()
		if err != nil {
			b.Fatal(err)
		}
		solvers[i], err = contexts[i].NewSolver()
		if err != nil {
			b.Fatal(err)
		}
		if err := solvers[i].LoadSMT2String(input); err != nil {
			b.Fatal(err)
		}
	}
	loaded = heap() - base
	b.StartTimer()
	for _, s := range solvers {
		if mode != "Concrete" {
			prove := s.ProveUnsatByOrderAbstraction
			if mode == "OrderBatch" {
				batch := s.BeginOrderProofBatch()
				batches = append(batches, batch)
				prove = batch.Prove
			}
			if proved, err := prove(); err != nil || !proved {
				b.Fatalf("order proof = %t, err %v", proved, err)
			}
			continue
		}
		if err := s.Push(); err != nil {
			b.Fatal(err)
		}
		if answer, err := s.Check(); err != nil || answer != lbool.False {
			b.Fatalf("concrete check = %v, err %v", answer, err)
		}
		if err := s.Pop(1); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if len(batches) > 0 {
		active = heap() - base
		for _, batch := range batches {
			if err := batch.Close(); err != nil {
				b.Fatal(err)
			}
		}
		batches = nil
	}
	idle = heap() - base
	for _, c := range contexts {
		m, err := c.Manager()
		if err != nil {
			b.Fatal(err)
		}
		budget += float64(m.Budget().Used())
	}
	return loaded, active, idle, budget
}
