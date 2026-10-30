package nlsat

import "testing"

func TestEmptyClauseDeletionDetachesFalseLiteralWatch(t *testing.T) {
	s := solverFixture(t)
	before := len(s.atoms.bools.watches[TrueBooleanVariable])
	clause, err := s.clauses.mkClause(nil, true, nil)
	assignmentMust(t, err)
	if len(s.atoms.bools.watches[TrueBooleanVariable]) != before+1 {
		t.Fatal("false literal watch", before, len(s.atoms.bools.watches[TrueBooleanVariable]))
	}
	assignmentMust(t, s.clauses.deleteClause(clause))
	if len(s.atoms.bools.watches[TrueBooleanVariable]) != before || s.clauses.owned[clause] != nil {
		t.Fatal("false literal watch not detached", before, len(s.atoms.bools.watches[TrueBooleanVariable]))
	}
}
