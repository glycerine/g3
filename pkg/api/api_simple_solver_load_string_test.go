package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverLoadSMT2String(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	astTestMust(t, s.LoadSMT2String("(declare-const x Int)(assert (> x 3))(assert (< x 3))"))
	if status, checkErr := s.Check(); checkErr != nil || status != lbool.False {
		t.Fatal("status", status, checkErr)
	}
	astTestMust(t, s.Close())
}
