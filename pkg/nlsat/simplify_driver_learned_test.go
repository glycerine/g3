package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyDriverDeletesLearnedPrefixBeforeRounds(t *testing.T) {
	s := solverFixture(t)
	b, err := s.MkBoolVar()
	assignmentMust(t, err)
	learned, err := s.clauses.mkClause([]sat.Literal{sat.MakeLiteral(b, false)}, true, nil)
	assignmentMust(t, err)
	summary, err := s.Simplify()
	assignmentMust(t, err)
	if summary.LearnedRemoved != 1 || len(s.clauses.learned) != 0 || s.clauses.owned[learned] != nil {
		t.Fatal(summary, len(s.clauses.learned))
	}
}
