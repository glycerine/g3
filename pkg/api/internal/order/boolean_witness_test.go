package order

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func witnessLimit(t testing.TB) *control.Reslimit {
	t.Helper()
	limit, err := control.NewReslimit(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(limit.Close)
	return limit
}

func TestBooleanWitnessAgainstTruthTable(t *testing.T) {
	limit := witnessLimit(t)
	// Exhaust every subset of these binary and ternary clauses. Include
	// duplicate literals, opposite signs and variables forced by a decision.
	rows := []BooleanClause{{1, 2}, {1, -2}, {-1, 2}, {-1, -2}, {3, 3}, {-3, -3}, {-1, -2, 3}, {1, 2, -3}, {-1, 1}, {-2, 3, 2}}
	for mask := 1; mask < 1<<len(rows); mask++ {
		k := BooleanProblem{Vars: 3}
		for i, row := range rows {
			if mask>>i&1 != 0 {
				k.Clauses = append(k.Clauses, row)
			}
		}
		if !k.HasSatisfyingAssignment(limit) {
			continue
		}
		sat := false
		for assignment := 0; assignment < 8; assignment++ {
			all := true
			for _, row := range k.Clauses {
				yes := false
				for _, lit := range row.literals() {
					v := lit
					if v < 0 {
						v = -v
					}
					if (assignment>>(v-1)&1 != 0) == (lit > 0) {
						yes = true
						break
					}
				}
				if !yes {
					all = false
					break
				}
			}
			if all {
				sat = true
				break
			}
		}
		if !sat {
			t.Fatalf("claimed a witness for unsatisfiable clauses %v", k.Clauses)
		}
	}
}

func TestBooleanWitnessLimits(t *testing.T) {
	limit := witnessLimit(t)
	k := BooleanProblem{Vars: witnessMaxVars, Clauses: []BooleanClause{{1, witnessMaxVars}, {-1, -witnessMaxVars}}}
	original := slices.Clone(k.Clauses)
	if !k.HasSatisfyingAssignment(limit) {
		t.Fatal("missed boundary variable")
	}
	if err := limit.Push(1); err != nil {
		t.Fatal(err)
	}
	if k.HasSatisfyingAssignment(limit) {
		t.Fatal("accepted over-budget assignment")
	}
	limit.Pop()
	limit.IncCancel()
	if k.HasSatisfyingAssignment(limit) {
		t.Fatal("accepted cancelled assignment")
	}
	limit.DecCancel()
	if !slices.Equal(original, k.Clauses) {
		t.Fatal("witness attempt changed the SAT input")
	}
	k.Vars++
	if k.HasSatisfyingAssignment(limit) {
		t.Fatal("allocated beyond witness bounds")
	}
}

type witnessFixture struct {
	Name string
	BooleanProblem
}

func loadWitnessFixtures(t testing.TB) []witnessFixture {
	t.Helper()
	data, err := os.ReadFile("test_vectors/hybrid_boolean_residuals.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []witnessFixture
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}
func TestBooleanWitnessHybridResiduals(t *testing.T) {
	limit := witnessLimit(t)
	for _, f := range loadWitnessFixtures(t) {
		if got := f.HasSatisfyingAssignment(limit); got != (f.Name != "Refuted") {
			t.Fatalf("%s returned %v", f.Name, got)
		}
	}
}
func BenchmarkBooleanWitness(b *testing.B) {
	for _, f := range loadWitnessFixtures(b) {
		b.Run(f.Name, func(b *testing.B) {
			limit := witnessLimit(b)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if got := f.HasSatisfyingAssignment(limit); got != (f.Name != "Refuted") {
					b.Fatal(got)
				}
			}
		})
	}
}
