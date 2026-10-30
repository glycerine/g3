package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestEmptyInputClauseMakesSolverUnsatisfiable(t *testing.T) {
	s := solverFixture(t)
	assignmentMust(t, s.AddClause(nil, Assumption{}))
	status, err := s.Check()
	assignmentMust(t, err)
	if status != lbool.False {
		t.Fatal("empty clause status", status)
	}
}
