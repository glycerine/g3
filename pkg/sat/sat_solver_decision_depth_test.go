package sat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATDecisionDepthAcrossAssumptions(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	for range 12 {
		_, err := s.MkVar(true, true)
		satOK(t, err)
	}
	x := MakeLiteral(0, false)
	satOK(t, s.MkClause([]Literal{x}))
	check := func(lit Literal, depth uint32, want lbool.LBool) {
		t.Helper()
		got, err := s.CheckWithDecisionDepth([]Literal{lit}, depth)
		satOK(t, err)
		if got != want {
			t.Fatalf("assumption %v, depth %d: got %v, want %v", lit, depth, got, want)
		}
		satOK(t, s.PopToBaseLevel())
	}
	check(x, 1, lbool.Undef)
	check(x.Negated(), 1, lbool.False)
	check(x, 0, lbool.True)
	check(x, 1, lbool.Undef)
	got, err := s.Check(nil)
	satOK(t, err)
	if got != lbool.True {
		t.Fatal(got)
	}
	copy := *s
	if _, err := copy.CheckWithDecisionDepth(nil, 1); !errors.Is(err, ErrSATOwner) {
		t.Fatal(err)
	}
}

func TestSATDecisionDepthChecksRefutationsBeforeBranching(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		s, _, _ := satSolverCheckFixture(t)
		for range 2 {
			_, e := s.MkVar(true, true)
			satOK(t, e)
		}
		var cs [][]Literal
		for i := 0; i < 4; i++ {
			if mask&(1<<i) == 0 {
				continue
			}
			cl := []Literal{MakeLiteral(0, i&1 != 0), MakeLiteral(1, i&2 != 0)}
			cs = append(cs, cl)
			satOK(t, s.MkClause(cl))
		}
		want := satSolverFiniteCNFOracle(cs, 2)
		got, err := s.CheckWithDecisionDepth(nil, 1)
		satOK(t, err)
		if got != lbool.Undef && got != want {
			t.Fatalf("formula %d, bounded result %v, want %v", mask, got, want)
		}
		got, err = s.Check(nil)
		satOK(t, err)
		if got != want {
			t.Fatalf("formula %d, ordinary search after bounded search: got %v, want %v", mask, got, want)
		}
	}
}

func TestSATDecisionDepthAppliesToPortfolioLanes(t *testing.T) {
	s := satParallelPortfolioFixture(t, 2)
	for range 12 {
		_, err := s.MkVar(true, true)
		satOK(t, err)
	}
	got, err := s.CheckWithDecisionDepth(nil, 1)
	satOK(t, err)
	if got != lbool.Undef {
		t.Fatalf("bounded portfolio returned %v", got)
	}
	got, err = s.Check(nil)
	satOK(t, err)
	if got != lbool.True {
		t.Fatalf("unbounded portfolio returned %v", got)
	}
}
