package api

import (
	"errors"
	"testing"
)

func TestAPISimpleSolverLifecycleScopesAndOwningAssertions(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-simple-solver-p", boolean))
	astTestMust(t, s.Assert(p))
	astTestMust(t, s.Push())
	if level, e := s.ScopeLevel(); e != nil || level != 1 {
		t.Fatal("scope", level, e)
	}
	assertions, err := s.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("assertion count", len(assertions))
	}
	astTestMust(t, s.Pop(1))
	astTestMust(t, s.Close())
	if _, err = s.ScopeLevel(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed solver", err)
	}
	if got, e := f.m.RefGet(assertions[0]); e != nil || got != p {
		t.Fatal("assertion vector ownership", got, e)
	}
	astTestMust(t, f.m.DelRef(&assertions[0]))
	if _, err = f.m.RefGet(assertions[0]); err == nil {
		t.Fatal("released assertion reference", err)
	}
}
