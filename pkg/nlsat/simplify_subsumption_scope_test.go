package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifySubsumptionLeavesBooleanOnlyClausesOutsideOccurrenceIndex(t *testing.T) {
	s := solverFixture(t)
	a, err := s.MkBoolVar()
	assignmentMust(t, err)
	b, err := s.MkBoolVar()
	assignmentMust(t, err)
	clause := []sat.Literal{sat.MakeLiteral(a, false), sat.MakeLiteral(b, false)}
	assignmentMust(t, s.AddClause(clause, Assumption{}))
	assignmentMust(t, s.AddClause(clause, Assumption{}))
	count, err := s.SimplifySubsumedClauses()
	assignmentMust(t, err)
	if count != 0 || len(s.clauses.originals) != 3 {
		t.Fatal("Boolean-only clauses entered arithmetic occurrence pass", count, len(s.clauses.originals))
	}
}
