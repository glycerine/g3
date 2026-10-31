package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIQFNRANLSATNamedTacticModelAndScopes(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_NRA", nil)
	tac, err := f.c.Tactic("qfnra-nlsat")
	astTestMust(t, err)
	s, err := f.c.NewSolverFromTactic(tac)
	astTestMust(t, err)
	astTestMust(t, tac.Close())
	defer func() { astTestMust(t, s.Close()) }()
	astTestMust(t, s.LoadSMT2String(`(declare-const x Real) (assert (= (* x x) 4))`))
	status, err := s.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("satisfiable nonlinear goal", status)
	}
	refs, err := s.Assertions()
	astTestMust(t, err)
	defer releaseAPIRefs(t, f.m, refs)
	formula, err := f.m.RefGet(refs[0])
	astTestMust(t, err)
	md, err := s.Model()
	astTestMust(t, err)
	value, err := md.Evaluate(formula, true)
	astTestMust(t, err)
	h, err := f.m.RefGet(value)
	astTestMust(t, err)
	if !f.m.IsTrue(h) {
		t.Fatal("nonlinear model", h)
	}
	astTestMust(t, f.m.DelRef(&value))
	astTestMust(t, md.Close())
	astTestMust(t, s.Push())
	neg, err := f.c.Not(formula)
	astTestMust(t, err)
	astTestMust(t, s.Assert(neg))
	status, err = s.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("contradictory scope", status)
	}
	astTestMust(t, s.Pop(1))
	status, err = s.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("restored scope", status)
	}
}
