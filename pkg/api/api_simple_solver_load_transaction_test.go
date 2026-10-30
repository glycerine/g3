package api

import (
	"strings"
	"testing"
)

func TestAPISimpleSolverLoadSMT2ParseFailureIsAtomic(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-load-transaction-p", boolean))
	astTestMust(t, s.Assert(p))
	if err = s.LoadSMT2(strings.NewReader("(assert false)(assert")); err == nil {
		t.Fatal("invalid stream accepted")
	}
	assertions, err := s.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("parse failure mutated solver", len(assertions))
	}
	astTestMust(t, f.m.DelRef(&assertions[0]))
	astTestMust(t, s.Close())
}
