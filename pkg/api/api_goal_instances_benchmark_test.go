package api

import (
	"compress/gzip"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Each process in the GoIvy comparison retains the original solver's
// incremental history. This fixture instead starts a new solver per operation
// so subsequent benchmark iterations cannot benefit from a prior result.
func BenchmarkAppleGoalInstances(b *testing.B) {
	for _, check := range []string{"4", "13", "22"} {
		input := appleBenchmarkInput(b, "rfn2-"+check)
		for _, mode := range []string{"Concrete", "GroundInstances"} {
			b.Run("Check"+check+"/"+mode, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if err := benchmarkGoalInstances(b, input, mode == "GroundInstances"); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// The captured check omits the preceding incremental history. Fresh measures
// parsing and setup as well as search. Prepared times only the independent
// proof attempt on the already loaded source assertions.
func BenchmarkAppleQuantifiedGoal(b *testing.B) {
	input := appleBenchmarkInput(b, "dramc_nb2-71")
	for _, mode := range []string{"Concrete", "Projection"} {
		b.Run("Fresh/"+mode, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if err := benchmarkGoalInstances(b, input, mode == "Projection"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	b.Run("Prepared/Projection", func(b *testing.B) {
		c, err := NewSMTContext()
		if err != nil {
			b.Fatal(err)
		}
		defer c.Close()
		s, err := c.NewSolver()
		if err != nil {
			b.Fatal(err)
		}
		defer s.Close()
		if err = s.LoadSMT2String(input); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if proved, err := s.ProveUnsatByOrderAbstraction(); !proved || err != nil {
				b.Fatalf("projected proof returned %v, %v", proved, err)
			}
		}
		b.StopTimer()
	})
}

// The same captured VC as BenchmarkAppleQuantifiedGoal, with projection and
// SAT search outside the measurement. Each operation starts from the original
// clauses. Copying them restores the input consumed by Reduce.
func BenchmarkAppleBooleanUnitReduction(b *testing.B) {
	input := appleBenchmarkInput(b, "dramc_nb2-71")
	c, err := NewSMTContext()
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()
	s, err := c.NewSolver()
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	if err = s.LoadSMT2String(input); err != nil {
		b.Fatal(err)
	}
	roots, err := solver.Assertions(s.solver)
	if err != nil {
		b.Fatal(err)
	}
	m, err := c.Manager()
	if err != nil {
		b.Fatal(err)
	}
	original, err := order.ProjectQuantifiedGoal(m, roots, orderProofSteps)
	if err != nil || original == nil {
		b.Fatalf("constructing the quantified Boolean problem failed: %v", err)
	}
	rows := make([]order.BooleanClause, len(original.Clauses))
	limit := m.ResourceLimit()
	var remaining int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(rows, original.Clauses)
		k := order.BooleanProblem{Vars: original.Vars, Clauses: rows}
		conflict, complete := k.Reduce(16, limit)
		if !complete || conflict {
			b.Fatalf("expected a residual SAT problem, got conflict=%t complete=%t", conflict, complete)
		}
		remaining = len(k.Clauses)
	}
	b.StopTimer()
	b.ReportMetric(float64(len(rows)), "input-clauses/op")
	b.ReportMetric(float64(remaining), "residual-clauses/op")
}

func appleBenchmarkInput(b *testing.B, name string) string {
	b.Helper()
	f, err := os.Open("../perf/ivy_apple/test_vectors/" + name + ".smt2.gz")
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
	input, _, ok := strings.Cut(string(data), "\n(check-sat)")
	if !ok {
		b.Fatal("captured input is missing check-sat")
	}
	return input
}

func benchmarkGoalInstances(b *testing.B, input string, useInstances bool) error {
	c, err := NewSMTContext()
	if err != nil {
		return err
	}
	defer c.Close()
	s, err := c.NewSolver()
	if err != nil {
		return err
	}
	defer s.Close()
	if err = s.LoadSMT2String(input); err != nil {
		return err
	}
	if useInstances {
		proved, err := s.ProveUnsatByOrderAbstraction()
		if err == nil && !proved {
			b.Fatal("goal-instance proof was inconclusive")
		}
		return err
	}
	answer, err := s.Check()
	if err == nil && answer != lbool.False {
		b.Fatalf("concrete query returned %v", answer)
	}
	return err
}
