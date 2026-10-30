package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverLoadSMT2UsesFinalParserScope(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	input := "(declare-const p Bool)(assert p)(push 1)(assert false)(pop 1)"
	astTestMust(t, s.LoadSMT2(strings.NewReader(input)))
	assertions, err := s.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("temporary parser scope leaked", len(assertions))
	}
	astTestMust(t, f.m.DelRef(&assertions[0]))
	if status, checkErr := s.Check(); checkErr != nil || status != lbool.True {
		t.Fatal("status", status, checkErr)
	}
	astTestMust(t, s.Close())
}
