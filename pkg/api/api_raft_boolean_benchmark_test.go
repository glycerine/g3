package api

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"testing"
)

func oneBitRaftQueries(b *testing.B) []string {
	b.Helper()
	return raftProofQueries(b, "onebit")
}

func raftProofQueries(b *testing.B, width string) []string {
	b.Helper()
	f, err := os.Open("test_vectors/raft_order/" + width + ".jsonl.gz")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		b.Fatal(err)
	}
	defer z.Close()
	dec := json.NewDecoder(z)
	var queries []string
	for {
		var input [2]string
		err := dec.Decode(&input)
		if err == io.EOF {
			break
		}
		if err != nil || input[0] != fmt.Sprintf("%03d.smt2", len(queries)+1) {
			b.Fatal(input[0], err)
		}
		queries = append(queries, input[1])
	}
	if len(queries) != 419 {
		b.Fatalf("read %d Raft obligations", len(queries))
	}
	return queries
}

// Each iteration loads and proves the complete sequence on a single solver.
// It includes parsing and scope changes, with the same inputs in both modes.
func BenchmarkRaftBooleanProofs(b *testing.B) {
	queries := oneBitRaftQueries(b)
	for _, reuse := range []bool{false, true} {
		name := "Fresh"
		if reuse {
			name = "Batch"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := runRaftProofQueries(queries, reuse); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(len(queries)), "obligations/op")
		})
	}
}

// Each worker owns a context, a solver and a proof batch from creation to close.
// Contiguous ranges preserve premise reuse within each range. Every round
// checks all 419 input obligations, including parsing and scope changes.
func BenchmarkRaftIndependentProofs(b *testing.B) {
	for _, width := range []string{"mixed", "onebit"} {
		queries := raftProofQueries(b, width)
		for _, workers := range []int{1, 4, 16, 64} {
			b.Run(width+"/"+strconv.Itoa(workers), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					errs := make([]error, workers)
					var group sync.WaitGroup
					for worker := 0; worker < workers; worker++ {
						group.Add(1)
						go func(w int) {
							defer group.Done()
							start, end := w*len(queries)/workers, (w+1)*len(queries)/workers
							errs[w] = runRaftProofQueries(queries[start:end], true)
						}(worker)
					}
					group.Wait()
					for w, err := range errs {
						if err != nil {
							b.Fatalf("worker %d: %v", w, err)
						}
					}
				}
				b.ReportMetric(float64(len(queries)), "obligations/op")
			})
		}
	}
}

func runRaftProofQueries(queries []string, reuse bool) (err error) {
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
	prove := source.ProveUnsatByOrderAbstraction
	if reuse {
		batch := source.BeginOrderProofBatch()
		defer func() { err = errors.Join(err, batch.Close()) }()
		prove = batch.Prove
	}
	for i, input := range queries {
		if err := source.Push(); err != nil {
			return err
		}
		if err := source.LoadSMT2String(input); err != nil {
			return err
		}
		proved, err := prove()
		if err != nil {
			return err
		}
		if !proved {
			return fmt.Errorf("unproved query %d in batch", i+1)
		}
		if err := source.Pop(1); err != nil {
			return err
		}
	}
	return nil
}
