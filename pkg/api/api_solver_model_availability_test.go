package api

// Z3_solver_get_model after Z3_solver_check for the simple solver and the
// default (combined) solver, with and without a push, and with the context
// parameter model=false. smt_solver ignores set_produce_models and builds
// its model from smt_params::m_model (always true here), so it answers with
// a model in every case; only combined_solver's non-incremental tactic
// solver honors model=false. Expected results come from the C++ z3 4.7.1
// release library (build/libz3.a) through a C harness running the same
// calls:
//
//	simple: 1 x -> 7          simple_push: 1 x -> 7
//	combined: 1 x -> 7        combined_push: 1 x -> 7
//	simple_nomodel: 1 x -> 7  combined_push_nomodel: 1 x -> 7
//	combined_nomodel: Z3_INVALID_USAGE (no model)

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSolverModelAvailabilityMatchesCPP(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		combined, model, push bool
		wantModel             bool
	}{
		{"simple", false, true, false, true},
		{"simple_push", false, true, true, true},
		{"combined", true, true, false, true},
		{"combined_push", true, true, true, true},
		{"simple_nomodel", false, false, false, true},
		{"combined_push_nomodel", true, false, true, true},
		{"combined_nomodel", true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewSMTContext()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, c.Close()) })
			if !tc.model {
				astTestMust(t, c.SetBoolOption("model", false))
			}
			m, err := c.Manager()
			astTestMust(t, err)
			f := &smtAPIFixture{t: t, c: c, m: m}
			var s *APISimpleSolver
			if tc.combined {
				s, err = c.NewSolver()
			} else {
				s, err = c.NewSimpleSolver()
			}
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()) })
			intSort := f.pin(f.c.IntegerSort())
			x := f.pin(f.c.Constant("x", intSort))
			if tc.push {
				astTestMust(t, s.Push())
			}
			astTestMust(t, s.Assert(f.pin(f.c.Equal(x, f.pin(f.c.IntNumeral(7, intSort))))))
			status, err := s.Check()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("status", status)
			}
			md, err := s.Model()
			if !tc.wantModel {
				if err == nil {
					astTestMust(t, md.Close())
					t.Fatal("C++ reports no model (invalid usage)")
				}
				return
			}
			astTestMust(t, err)
			text, err := md.String()
			astTestMust(t, err)
			astTestMust(t, md.Close())
			if strings.TrimSpace(text) != "x -> 7" {
				t.Fatalf("model %q, C++ prints %q", text, "x -> 7")
			}
		})
	}
}
