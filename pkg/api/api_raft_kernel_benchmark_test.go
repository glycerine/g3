package api

import (
	"compress/gzip"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// BenchmarkRaftOrderKernel replays the exact order problems submitted to
// both SMT kernels. Each check gets a fresh context and the same parameters.
// Decode the fixture outside the measured interval.
func BenchmarkRaftOrderKernel(b *testing.B) {
	f, err := os.Open("test_vectors/raft_order/kernel.smt2.gz")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		b.Fatal(err)
	}
	defer z.Close()
	data, err := io.ReadAll(z)
	if err != nil {
		b.Fatal(err)
	}
	const boundary = "(check-sat)\n(reset)\n"
	body, ok := strings.CutSuffix(string(data), boundary)
	queries := strings.Split(body, boundary)
	if !ok || len(queries) != 419 {
		b.Fatal("incomplete Raft corpus", len(queries))
	}
	var load, check time.Duration
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j, query := range queries {
			l, c, err := replayRaftOrderQuery(query)
			if err != nil {
				b.Fatalf("obligation %d: %v", j+1, err)
			}
			load += l
			check += c
		}
	}
	b.ReportMetric(float64(load.Nanoseconds())/float64(b.N), "load-ns/op")
	b.ReportMetric(float64(check.Nanoseconds())/float64(b.N), "check-ns/op")
	b.ReportMetric(float64(len(queries)), "obligations/op")
}

func replayRaftOrderQuery(query string) (load, check time.Duration, err error) {
	ctx, err := NewSMTContext()
	if err != nil {
		return 0, 0, err
	}
	defer func() { err = errors.Join(err, ctx.Close()) }()
	s, err := ctx.NewSimpleSolver()
	if err != nil {
		return 0, 0, err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	params, err := ctx.NewParameterSet()
	if err != nil {
		return 0, 0, err
	}
	defer func() { err = errors.Join(err, params.Close()) }()
	if err = params.SetBool("smt.macro_finder", true); err != nil {
		return 0, 0, err
	}
	if err = s.SetParameters(params); err != nil {
		return 0, 0, err
	}
	start := time.Now()
	if err = s.LoadSMT2String(query); err != nil {
		return 0, 0, err
	}
	loaded := time.Now()
	answer, err := s.Check()
	if err == nil && answer != lbool.False {
		err = errors.New("expected UNSAT")
	}
	return loaded.Sub(start), time.Since(loaded), err
}
