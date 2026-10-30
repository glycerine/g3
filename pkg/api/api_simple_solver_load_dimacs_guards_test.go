package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/sat"
)

func TestAPISimpleSolverLoadDIMACSFailureDoesNotMutateSolver(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-dimacs-existing", boolean))
	astTestMust(t, s.Assert(p))
	if err = s.LoadDIMACS(strings.NewReader("-1 missing 0")); !errors.Is(err, sat.ErrDIMACSParse) {
		t.Fatal("malformed DIMACS", err)
	}
	assertions, err := s.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("parse failure mutated solver", len(assertions))
	}
	astTestMust(t, f.m.DelRef(&assertions[0]))
	if err = s.LoadDIMACS(nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("nil reader", err)
	}
	astTestMust(t, s.Close())
}
