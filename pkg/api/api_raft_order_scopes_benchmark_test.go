package api

import (
	"compress/gzip"
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
)

type raftOrderScope struct {
	problem       order.Problem
	newKernel     bool
	batch         bool
	retainedBytes int
}

func raftOrderScopes(b *testing.B) []raftOrderScope {
	b.Helper()
	f, err := os.Open("test_vectors/raft_order/goivy-kernel.smt2.gz")
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
		b.Fatal("incomplete Raft formula corpus")
	}
	scopes, err := os.Open("test_vectors/raft_order/goivy-scopes.csv")
	if err != nil {
		b.Fatal(err)
	}
	defer scopes.Close()
	rows, err := csv.NewReader(scopes).ReadAll()
	if err != nil || len(rows) != len(queries)+1 {
		b.Fatal("incomplete Raft scope schedule", err)
	}
	result := make([]raftOrderScope, len(queries))
	for i, script := range queries {
		first := strings.Index(script, "(declare-")
		if first < 0 {
			b.Fatal("missing declarations", i+1)
		}
		script = script[first:]
		start := strings.Index(script, "(assert ")
		last := strings.LastIndex(script, "(assert ")
		if start < 0 || last < start {
			b.Fatal("missing assertions", i+1)
		}
		r := rows[i+1]
		if r[0] != strconv.Itoa(i+1) {
			b.Fatal("unexpected obligation", r[0])
		}
		n, err := strconv.Atoi(r[3])
		if err != nil {
			b.Fatal(err)
		}
		q := order.Problem{Text: script, AssertionsStart: start, LastAssertionStart: last,
			DeclarationCount: strings.Count(script[:start], "\n")}
		result[i] = raftOrderScope{problem: q, newKernel: r[1] == "true", batch: r[2] == "true", retainedBytes: n}
	}
	return result
}

// BenchmarkRaftOrderScopes feeds both policies the exact formulas emitted by
// GoIvy. Recorded follows the private kernel lifetimes of the full verifier.
func BenchmarkRaftOrderScopes(b *testing.B) {
	queries := raftOrderScopes(b)
	for _, test := range []struct {
		name  string
		fresh bool
	}{{"Fresh", true}, {"Recorded", false}} {
		b.Run(test.name, func(b *testing.B) {
			parent, err := NewSMTContext()
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() {
				if err := parent.Close(); err != nil {
					b.Error(err)
				}
			})
			m, err := parent.Manager()
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var proof orderProofState
				for j, query := range queries {
					if test.fresh || query.newKernel {
						if err := proof.release(); err != nil {
							b.Fatal(err)
						}
						if err := proof.start(m, orderSearch); err != nil {
							b.Fatal(err)
						}
					}
					reuse := !test.fresh && query.batch
					if reuse && proof.needsNewDeclarations(query.problem) {
						b.Fatal("incompatible recorded declarations", j+1)
					}
					ok, err := proof.check(m, query.problem, orderProofSteps, reuse)
					if err != nil || !ok {
						b.Fatalf("obligation %d, proved %v, error %v", j+1, ok, err)
					}
					if reuse && len(proof.premises) != query.retainedBytes {
						b.Fatal("unexpected retained premise", j+1)
					}
				}
				if err := proof.release(); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(len(queries)), "obligations/op")
		})
	}
}
