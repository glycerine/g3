package api

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

func oneBitRaftQueries(b *testing.B) []string {
	b.Helper()
	f, err := os.Open("test_vectors/raft_order/onebit.jsonl.gz")
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
				runRaftBooleanProofs(b, queries, reuse)
			}
			b.ReportMetric(float64(len(queries)), "obligations/op")
		})
	}
}

func runRaftBooleanProofs(b *testing.B, queries []string, reuse bool) {
	b.Helper()
	must := func(err error) {
		if err != nil {
			b.Fatal(err)
		}
	}
	ctx, err := NewSMTContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	s, err := ctx.NewSolver()
	must(err)
	defer func() { must(s.Close()) }()
	prove := s.ProveUnsatByOrderAbstraction
	if reuse {
		batch := s.BeginOrderProofBatch()
		defer func() { must(batch.Close()) }()
		prove = batch.Prove
	}
	for i, input := range queries {
		must(s.Push())
		must(s.LoadSMT2String(input))
		if proved, err := prove(); err != nil || !proved {
			b.Fatalf("obligation %03d proved %v, error %v", i+1, proved, err)
		}
		must(s.Pop(1))
	}
}
