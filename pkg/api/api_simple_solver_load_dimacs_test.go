package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverLoadDIMACSSatAndUnsat(t *testing.T) {
	for _, row := range []struct {
		name, input string
		want        lbool.LBool
	}{
		{"sat", "p cnf 2 2\n1 2 0\n-1 2 0\n", lbool.True},
		{"unsat", "p cnf 1 2\n1 0\n-1 0\n", lbool.False},
	} {
		t.Run(row.name, func(t *testing.T) {
			f := newSMTAPIFixture(t, "QF_UF", nil)
			s, err := f.c.NewSimpleSolver()
			astTestMust(t, err)
			astTestMust(t, s.LoadDIMACS(strings.NewReader(row.input)))
			if status, checkErr := s.Check(); checkErr != nil || status != row.want {
				t.Fatal("status", status, checkErr)
			}
			astTestMust(t, s.Close())
		})
	}
}
