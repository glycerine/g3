package nlsat

import "testing"

func TestEmptyInputClauseNormalizesToFalseLiteral(t *testing.T) {
	s := solverFixture(t)
	assignmentMust(t, s.AddClause(nil, Assumption{}))
	clause := s.clauses.originals[len(s.clauses.originals)-1]
	if clause.Size() != 1 || clause.Literals()[0] != FalseLiteral {
		t.Fatal("normalized empty input", clause.Literals())
	}
}
