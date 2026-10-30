package nlsat

import "testing"

func TestEmptyLearnedClauseNormalizesBeforePublication(t *testing.T) {
	s := solverFixture(t)
	clause, err := s.clauses.mkClause(nil, true, nil)
	assignmentMust(t, err)
	if clause.Size() != 1 || clause.Literals()[0] != FalseLiteral || !clause.IsLearned() || s.clauses.learned[len(s.clauses.learned)-1] != clause {
		t.Fatal("normalized learned clause", clause.Literals(), clause.IsLearned())
	}
}
