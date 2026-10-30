package nlsat

import "testing"

func TestEmptyClauseAttachesAsFalseBooleanWatch(t *testing.T) {
	s := solverFixture(t)
	clause, err := s.clauses.mkClause(nil, true, nil)
	assignmentMust(t, err)
	watches := s.atoms.bools.watches[TrueBooleanVariable]
	found := false
	for _, watched := range watches {
		found = found || watched == clause
	}
	if !found {
		t.Fatal("normalized false clause not attached", len(watches))
	}
}
