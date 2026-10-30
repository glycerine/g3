package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverLoadSMT2ReaderAddsAssertions(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	astTestMust(t, s.LoadSMT2(strings.NewReader("(declare-const p Bool)(assert p)(assert (not p))")))
	assertions, err := s.Assertions()
	astTestMust(t, err)
	if len(assertions) != 2 {
		t.Fatal("assertion count", len(assertions))
	}
	for i := range assertions {
		astTestMust(t, f.m.DelRef(&assertions[i]))
	}
	if status, checkErr := s.Check(); checkErr != nil || status != lbool.False {
		t.Fatal("status", status, checkErr)
	}
	astTestMust(t, s.Close())
}
