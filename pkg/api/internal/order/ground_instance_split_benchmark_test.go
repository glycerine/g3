package order

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/control"
)

func readGroundSplitWitness(b *testing.B, name string) GroundInstanceQuery {
	b.Helper()
	path := filepath.Join("..", "..", "..", "perf", "ivy_apple", "test_vectors", name+"_observed_split.cnf.gz")
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
	r := bufio.NewReader(z)
	comment, err := r.ReadString('\n')
	if err != nil {
		b.Fatal(err)
	}
	q := GroundInstanceQuery{units: new(instanceUnits)}
	if _, err := fmt.Sscanf(comment, "c goal %d", &q.Goal); err != nil {
		b.Fatal(err)
	}
	cnf, err := sat.ReadDIMACSCNF(r)
	if err != nil {
		b.Fatal(err)
	}
	q.Variables = int(cnf.VariableCount)
	for _, clause := range cnf.Clauses {
		var row BooleanClause
		if len(clause) > len(row) {
			b.Fatalf("clause has %d literals", len(clause))
		}
		for i, lit := range clause {
			row[i] = int32(lit)
		}
		q.Clauses = append(q.Clauses, row)
	}
	return q
}

func BenchmarkGroundInstanceObservedSplit(b *testing.B) {
	for _, name := range []string{"alternating_bit_protocol", "hermes_rmw_o3"} {
		b.Run(name, func(b *testing.B) {
			q := readGroundSplitWitness(b, name)
			limit, err := control.NewReslimit(nil)
			if err != nil {
				b.Fatal(err)
			}
			defer limit.Close()
			q.Goal = -q.Goal
			if conflict, ok := q.Reduce(limit, true); conflict || !ok {
				b.Fatal("satisfiable opposite goal")
			}
			q.Goal = -q.Goal
			for _, mode := range []struct {
				name  string
				split bool
			}{{"Units", false}, {"WithSplits", true}} {
				b.Run(mode.name, func(b *testing.B) {
					start := limit.Count()
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if conflict, ok := q.Reduce(limit, mode.split); conflict != mode.split || !ok {
							b.Fatalf("conflict %v, complete %v", conflict, ok)
						}
					}
					b.StopTimer()
					b.ReportMetric(float64(limit.Count()-start)/float64(b.N), "steps/op")
				})
			}
		})
	}
}
