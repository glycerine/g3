package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISMTTacticReentrantArithmeticITEAtoms(t *testing.T) {
	for _, tc := range []struct {
		name, assumption string
		want             lbool.LBool
	}{
		{"sat", "p", lbool.True}, {"unsat", "(not p)", lbool.False},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSMTAPIFixture(t, "ALL", nil)
			tac, err := f.c.Tactic("smt")
			astTestMust(t, err)
			s, err := f.c.NewSolverFromTactic(tac)
			astTestMust(t, err)
			astTestMust(t, tac.Close())
			defer func() { astTestMust(t, s.Close()) }()
			astTestMust(t, s.LoadSMT2String(`(declare-const p Bool) (assert (<= 0 (ite p 1 0))) (assert (<= (ite p 1 0) 1)) (assert `+tc.assumption+`) (assert (>= (ite p 1 0) 1))`))
			got, err := s.Check()
			astTestMust(t, err)
			if got != tc.want {
				reason, _ := s.ReasonUnknown()
				t.Fatal("result", got, "want", tc.want, reason)
			}
		})
	}
}
