package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverLoadSMT2IgnoresCheckCommands(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	input := "(declare-const p Bool)(assert p)(check-sat)(assert (not p))(check-sat)"
	astTestMust(t, s.LoadSMT2(strings.NewReader(input)))
	if status, checkErr := s.Check(); checkErr != nil || status != lbool.False {
		t.Fatal("loaded checks changed final assertions", status, checkErr)
	}
	astTestMust(t, s.Close())
}
