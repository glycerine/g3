package nlsat

import "testing"

func TestEmptyClauseConsumesOneClauseIdentifier(t *testing.T) {
	s := solverFixture(t)
	first, err := s.clauses.mkClause(nil, true, nil)
	assignmentMust(t, err)
	second, err := s.clauses.mkClause(nil, true, nil)
	assignmentMust(t, err)
	if second.ID() != first.ID()+1 {
		t.Fatal("empty clause identifiers", first.ID(), second.ID())
	}
}
