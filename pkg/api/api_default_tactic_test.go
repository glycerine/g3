package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIDefaultTacticSourceBranches(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		want         lbool.LBool
	}{
		{"bv", `(declare-const x (_ BitVec 4)) (assert (= (bvadd x #x1) #x3))`, lbool.True},
		{"array_bv", `(declare-const a (Array (_ BitVec 2) (_ BitVec 2))) (assert (not (= (select a #b00) (select a #b00))))`, lbool.False},
		{"integer", `(declare-const x Int) (assert (> x 1))`, lbool.True},
		{"real", `(declare-const x Real) (assert (and (> x 1) (< x 0)))`, lbool.False},
		{"nonlinear", `(declare-const x Real) (assert (= (* x x) 4))`, lbool.True},
		{"uninterpreted", `(declare-const p Bool) (assert p)`, lbool.True},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSMTAPIFixture(t, "ALL", nil)
			tac, err := f.c.Tactic("default")
			astTestMust(t, err)
			s, err := f.c.NewSolverFromTactic(tac)
			astTestMust(t, err)
			astTestMust(t, tac.Close())
			defer func() { astTestMust(t, s.Close()) }()
			astTestMust(t, s.LoadSMT2String(tc.script))
			got, err := s.Check()
			astTestMust(t, err)
			if got != tc.want {
				reason, _ := s.ReasonUnknown()
				t.Fatalf("got %v, want %v: %s", got, tc.want, reason)
			}
			if got == lbool.True {
				refs, err := s.Assertions()
				astTestMust(t, err)
				defer releaseAPIRefs(t, f.m, refs)
				model, err := s.Model()
				astTestMust(t, err)
				defer func() { astTestMust(t, model.Close()) }()
				for _, ref := range refs {
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
