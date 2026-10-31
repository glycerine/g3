package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIMoreRegisteredTacticFactories(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		solver       bool
	}{
		{"sine-filter", `(declare-const p Bool) (assert p)`, false},
		{"elim01", `(declare-const x Int) (assert (and (<= 0 x) (<= x 1) (= x 1)))`, false},
		{"bvarray2uf", `(declare-const a (Array (_ BitVec 2) (_ BitVec 2))) (assert (= (select a #b00) #b01))`, false},
		{"nl-purify", `(declare-const x Real) (assert (= (* x x) 1))`, false},
		{"lra", `(declare-const x Real) (assert (> x 1))`, true},
		{"lia", `(declare-const x Int) (assert (> x 1))`, true},
		{"lira", `(declare-const x Int) (declare-const y Real) (assert (> y (to_real x)))`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSMTAPIFixture(t, "ALL", nil)
			found, err := f.c.HasTactic(tc.name)
			astTestMust(t, err)
			if !found {
				t.Fatal("missing catalog entry")
			}
			_, err = f.c.TacticDescription(tc.name)
			astTestMust(t, err)
			first, err := f.c.Tactic(tc.name)
			astTestMust(t, err)
			var combined *APITactic
			if !tc.solver {
				smt, err := f.c.Tactic("smt")
				astTestMust(t, err)
				combined, err = first.AndThen(smt)
				astTestMust(t, err)
				astTestMust(t, smt.Close())
			} else {
				combined, err = first.Retain()
				astTestMust(t, err)
			}
			s, err := f.c.NewSolverFromTactic(combined)
			astTestMust(t, err)
			astTestMust(t, combined.Close())
			astTestMust(t, first.Close())
			defer func() { astTestMust(t, s.Close()) }()
			astTestMust(t, s.LoadSMT2String(tc.script))
			status, err := s.Check()
			astTestMust(t, err)
			if tc.name == "sine-filter" {
				reason, err := s.ReasonUnknown()
				astTestMust(t, err)
				if status != lbool.Undef || reason != "over-approximated goal found to be sat" {
					t.Fatal("over-approximation result", status, reason)
				}
				return
			}
			if status != lbool.True {
				reason, _ := s.ReasonUnknown()
				t.Fatal("status", status, reason)
			}
			if tc.name == "elim01" || tc.name == "bvarray2uf" {
				assertions, err := s.Assertions()
				astTestMust(t, err)
				defer releaseAPIRefs(t, f.m, assertions)
				model, err := s.Model()
				astTestMust(t, err)
				defer func() { astTestMust(t, model.Close()) }()
				for _, ref := range assertions {
					formula, err := f.m.RefGet(ref)
					astTestMust(t, err)
					value, err := model.Evaluate(formula, true)
					astTestMust(t, err)
					actual, err := f.m.RefGet(value)
					astTestMust(t, err)
					if !f.m.IsTrue(actual) {
						t.Fatal("model violates original assertion", actual)
					}
					astTestMust(t, f.m.DelRef(&value))
				}
			}
		})
	}
}
