package api

import (
	"os"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// The checked-in SMT-LIB snapshot is independent of the external Ivy examples.
// Exercise both the tactic-backed solver and the incremental kernel used after
// a push. A work budget guards against runaway instantiation without relying on
// a tight, machine-dependent wall-clock performance assertion.
func TestAPIRaftLogMatching(t *testing.T) {
	for _, file := range []string{"raft_log_matching.smt2", "raft_log_matching_join.smt2"} {
		t.Run(file, func(t *testing.T) { testAPIRaftLogMatchingSnapshot(t, file) })
	}
}

func testAPIRaftLogMatchingSnapshot(t *testing.T, file string) {
	input, err := os.ReadFile("test_vectors/" + file)
	if err != nil {
		t.Fatal(err)
	}
	for _, incremental := range []bool{false, true} {
		name := "fresh"
		if incremental {
			name = "incremental"
		}
		t.Run(name, func(t *testing.T) {
			c, e := NewSMTContext()
			astTestMust(t, e)
			t.Cleanup(func() { astTestMust(t, c.Close()) })
			m, e := c.Manager()
			astTestMust(t, e)
			formula, e := c.ParseSMT2String(string(input), SMT2Bindings{})
			astTestMust(t, e)
			t.Cleanup(func() { astTestMust(t, m.DelRef(&formula)) })
			h, e := m.RefGet(formula)
			astTestMust(t, e)
			solver, e := c.NewSolver()
			astTestMust(t, e)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			params, e := c.NewParameterSet()
			astTestMust(t, e)
			t.Cleanup(func() { astTestMust(t, params.Close()) })
			astTestMust(t, params.SetBool("macro_finder", true))
			astTestMust(t, params.SetUInt("rlimit", 2000000))
			astTestMust(t, solver.SetParameters(params))
			if incremental {
				astTestMust(t, solver.Push())
			}
			astTestMust(t, solver.Assert(h))
			status, e := solver.Check()
			astTestMust(t, e)
			if status != lbool.False {
				reason, _ := solver.ReasonUnknown()
				t.Fatalf("log matching: got %v (%s), want unsat within resource budget", status, reason)
			}
			if incremental {
				astTestMust(t, solver.Pop(1))
			}
		})
	}
}
