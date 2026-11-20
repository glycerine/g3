package api

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// Run with -cpu=1 in a fresh process. Heap metrics use a full GC before each
// observation. The timer covers the proof on each independent solver.
func BenchmarkRaftIdleSolverMemory(b *testing.B) {
	input := raftMemoryInput(b)
	counts := raftMemoryOwners(b, []int{1, 8, 32, 64})
	for _, mode := range []string{"Concrete", "Order", "OrderBatch"} {
		for _, count := range counts {
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

// G3_RAFT_MEMORY_QUERY selects a captured query, for example mixed/274 or
// onebit/274. Keep the original single-file fixture for the default run.
func raftMemoryInput(b *testing.B) string {
	b.Helper()
	query := os.Getenv("G3_RAFT_MEMORY_QUERY")
	if query == "" {
		data, err := os.ReadFile("test_vectors/raft_order/001.smt2")
		if err != nil {
			b.Fatal(err)
		}
		return string(data)
	}
	width, number, ok := strings.Cut(query, "/")
	index, err := strconv.Atoi(number)
	if !ok || (width != "mixed" && width != "onebit") || err != nil || index < 1 || index > 419 {
		b.Fatalf("invalid G3_RAFT_MEMORY_QUERY %q", query)
	}
	return raftProofQueries(b, width)[index-1]
}

func raftMemoryOwners(b *testing.B, defaults []int) []int {
	b.Helper()
	if raw := os.Getenv("G3_RAFT_MEMORY_OWNERS"); raw != "" {
		count, err := strconv.Atoi(raw)
		if err != nil || count < 1 {
			b.Fatalf("invalid G3_RAFT_MEMORY_OWNERS %q", raw)
		}
		return []int{count}
	}
	return defaults
}

func raftLiveHeap() float64 {
	runtime.GC()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return float64(mem.HeapAlloc)
}

// All contexts remain resident until every worker has loaded the query.
// The timer covers proving and closing the entire group. Each owner lives on
// one goroutine from construction through cleanup.
func BenchmarkRaftResidentWorkerProofs(b *testing.B) {
	input := raftMemoryInput(b)
	for _, count := range raftMemoryOwners(b, []int{32}) {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			b.StopTimer()
			var resident float64
			for i := 0; i < b.N; i++ {
				base := raftLiveHeap()
				start := make(chan struct{})
				var loaded, finished sync.WaitGroup
				loaded.Add(count)
				finished.Add(count)
				errs := make([]error, count)
				for worker := range errs {
					go func(w int) {
						defer finished.Done()
						errs[w] = proveOnResidentWorker(input, &loaded, start)
					}(worker)
				}
				loaded.Wait()
				resident += raftLiveHeap() - base
				b.StartTimer()
				close(start)
				finished.Wait()
				b.StopTimer()
				for worker, err := range errs {
					if err != nil {
						b.Fatalf("worker %d: %v", worker, err)
					}
				}
			}
			b.ReportMetric(float64(count), "obligations/op")
			b.ReportMetric(resident/(float64(count)*float64(b.N)), "resident-B/solver")
		})
	}
}

func proveOnResidentWorker(input string, loaded *sync.WaitGroup, start <-chan struct{}) (err error) {
	// A failed setup still releases the coordinator's barrier.
	ready := false
	defer func() {
		if !ready {
			loaded.Done()
		}
	}()
	ctx, err := NewSMTContext()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, ctx.Close()) }()
	source, err := ctx.NewSolver()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, source.Close()) }()
	if err = source.LoadSMT2String(input); err != nil {
		return err
	}
	ready = true
	loaded.Done()
	<-start
	batch := source.BeginOrderProofBatch()
	defer func() { err = errors.Join(err, batch.Close()) }()
	proved, err := batch.Prove()
	if err == nil && !proved {
		return errors.New("expected Raft proof")
	}
	return err
}

func benchRaftIdleGroup(b *testing.B, input string, count int, mode string) (loaded, active, idle, budget float64) {
	base := raftLiveHeap()
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
	loaded = raftLiveHeap() - base
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
		active = raftLiveHeap() - base
		for _, batch := range batches {
			if err := batch.Close(); err != nil {
				b.Fatal(err)
			}
		}
		batches = nil
	}
	idle = raftLiveHeap() - base
	for _, c := range contexts {
		m, err := c.Manager()
		if err != nil {
			b.Fatal(err)
		}
		budget += float64(m.Budget().Used())
	}
	return loaded, active, idle, budget
}
