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
	queries := raftOrderKernelQueries(b)
	var load, check time.Duration
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j, query := range queries {
			l, c, err := replayRaftOrderQuery(query, nil)
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

func raftOrderKernelQueries(t testing.TB) []string {
	t.Helper()
	f, err := os.Open("test_vectors/raft_order/kernel.smt2.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	data, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	const boundary = "(check-sat)\n(reset)\n"
	body, ok := strings.CutSuffix(string(data), boundary)
	queries := strings.Split(body, boundary)
	if !ok || len(queries) != 419 {
		t.Fatal("incomplete Raft corpus", len(queries))
	}
	return queries
}

func replayRaftOrderQuery(query string, inspect func(*APISimpleSolver) error) (load, check time.Duration, err error) {
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
	checked := time.Now()
	if err == nil && answer != lbool.False {
		err = errors.New("expected UNSAT")
	}
	if err == nil && inspect != nil {
		err = inspect(s)
	}
	return loaded.Sub(start), checked.Sub(loaded), err
}
