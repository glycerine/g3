package api

import (
	"compress/gzip"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// Each process in the GoIvy comparison retains the original solver's
// incremental history. This fixture instead starts a new solver per operation
// so subsequent benchmark iterations cannot benefit from a prior result.
func BenchmarkAppleGoalInstances(b *testing.B) {
	for _, check := range []string{"4", "13", "22"} {
		file, err := os.Open("../perf/ivy_apple/test_vectors/rfn2-" + check + ".smt2.gz")
		if err != nil {
			b.Fatal(err)
		}
		compressed, err := gzip.NewReader(file)
		if err != nil {
			b.Fatal(err)
		}
		data, err := io.ReadAll(compressed)
		compressed.Close()
		file.Close()
		if err != nil {
			b.Fatal(err)
		}
		input := strings.TrimSuffix(strings.TrimSpace(string(data)), "(check-sat)")
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
