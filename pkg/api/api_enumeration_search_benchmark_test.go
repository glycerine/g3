package api

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
)

type enumerationReplayQuery struct {
	Declarations, Premises int
	Goal                   string
}

type enumerationReplayFixture struct {
	Declarations, Premises []string
	Batches                []struct {
		History []enumerationReplayQuery
		Core    enumerationReplayQuery
	}
}

func (f *enumerationReplayFixture) problem(q enumerationReplayQuery) order.Problem {
	decl, base := f.Declarations[q.Declarations], f.Premises[q.Premises]
	return order.Problem{
		Text:               decl + base + q.Goal,
		AssertionsStart:    len(decl),
		LastAssertionStart: len(decl) + len(base),
		DeclarationCount:   strings.Count(decl, "\n"),
	}
}

func loadEnumerationFixture(b *testing.B, path string) enumerationReplayFixture {
	b.Helper()
	f, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		b.Fatal(err)
	}
	defer z.Close()
	var fixture enumerationReplayFixture
	if err := json.NewDecoder(z).Decode(&fixture); err != nil {
		b.Fatal(err)
	}
	return fixture
}

// BenchmarkEnumerationSearch replays the six budget misses recorded on full
// ord_live2. Each saved history starts with a fresh private solver. The Core
// case checks a sufficient subset extracted and verified by in-tree Z3.
func BenchmarkEnumerationSearch(b *testing.B) {
	fixture := loadEnumerationFixture(b, "test_vectors/apple_enumeration_search.json.gz")
	if len(fixture.Batches) != 6 {
		b.Fatalf("got %d batches, want six recorded searches", len(fixture.Batches))
	}
	for i, batch := range fixture.Batches {
		history := make([]order.Problem, len(batch.History))
		for j, q := range batch.History {
			history[j] = fixture.problem(q)
		}
		for _, test := range []struct {
			name    string
			queries []order.Problem
		}{
			{"Warm", history},
			{"Cold", history[len(history)-1:]},
			{"Core", []order.Problem{fixture.problem(batch.Core)}},
		} {
			b.Run(strconv.Itoa(i+1)+"/"+test.name, func(b *testing.B) {
				benchmarkEnumerationSearch(b, test.queries)
			})
		}
	}
}

// Two live runs generated the same nine problems under different private
// symbol names. Replay the complete history or only its last goal.
func BenchmarkEnumerationRenamedHistory(b *testing.B) {
	fixture := loadEnumerationFixture(b, "test_vectors/apple_enumeration_renamed.json.gz")
	names := [...]string{"Original", "Renamed"}
	if len(fixture.Batches) != len(names) {
		b.Fatalf("got %d histories, want %d", len(fixture.Batches), len(names))
	}
	for i, batch := range fixture.Batches {
		if len(batch.History) != 9 {
			b.Fatalf("got %d queries in %s, want nine", len(batch.History), names[i])
		}
		history := make([]order.Problem, len(batch.History))
		for j, q := range batch.History {
			history[j] = fixture.problem(q)
		}
		b.Run(names[i]+"/Warm", func(b *testing.B) { benchmarkEnumerationSearch(b, history) })
		b.Run(names[i]+"/Cold", func(b *testing.B) { benchmarkEnumerationSearch(b, history[len(history)-1:]) })
	}
}

func benchmarkEnumerationSearch(b *testing.B, queries []order.Problem) {
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
	var goalsProved int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var proof orderProofState
		for j, q := range queries {
			if proof.ctx == nil {
				if err := proof.start(m, enumerationSearch); err != nil {
					b.Fatal(err)
				}
			}
			if proof.needsNewDeclarations(q) {
				b.Fatal("incompatible declarations within a captured kernel lifetime")
			}
			proved, err := proof.check(m, q, 6_000_000, true)
			if err != nil {
				b.Fatal(err)
			}
			if !proved || j == len(queries)-1 {
				if err := proof.release(); err != nil {
					b.Fatal(err)
				}
			}
			if j == len(queries)-1 && proved {
				goalsProved++
			}
		}
	}
	b.ReportMetric(float64(len(queries)), "checks/op")
	b.ReportMetric(float64(goalsProved)/float64(b.N), "final-proofs/op")
}
