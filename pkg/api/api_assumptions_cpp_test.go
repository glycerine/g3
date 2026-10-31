package api

// Source: src/test/check_assumptions.cpp and src/test/smt_context.cpp.
import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPICPPAssumptionRegressions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		three bool
	}{{"check_assumptions", true}, {"smt_context", false}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSMTAPIFixture(t, "QF_UF", nil)
			sort := f.pin(f.c.BoolSort())
			a := f.pin(f.c.Constant("cpp-assumption-a", sort))
			b := f.pin(f.c.Constant("cpp-assumption-b", sort))
			c := f.pin(f.c.Constant("cpp-assumption-c", sort))
			na := f.pin(f.c.Not(a))
			nb := f.pin(f.c.Not(b))
			nc := f.pin(f.c.Not(c))
			s, err := f.c.NewSimpleSolver()
			astTestMust(t, err)
			defer func() { astTestMust(t, s.Close()) }()
			if tc.three {
				clause := f.pin(f.c.Or(a, b, c))
				astTestMust(t, s.Assert(clause))
				got, err := s.Check(na)
				astTestMust(t, err)
				if got != lbool.True {
					t.Fatal("single assumption", got)
				}
				astTestMust(t, s.Assert(na))
				got, err = s.Check(nb, nc)
				astTestMust(t, err)
				if got != lbool.False {
					t.Fatal("contradictory assumptions", got)
				}
			} else {
				clause := f.pin(f.c.Or(c, b))
				astTestMust(t, s.Assert(na))
				astTestMust(t, s.Assert(clause))
				got, err := s.Check(nc)
				astTestMust(t, err)
				if got != lbool.True {
					t.Fatal("scoped assumption", got)
				}
			}
			got, err := s.Check()
			astTestMust(t, err)
			if got != lbool.True {
				t.Fatal("assumptions leaked into next check", got)
			}
		})
	}
}
