package nlsat

import "testing"

func TestEmptyInputClauseRetainsAssumption(t *testing.T) {
	s := solverFixture(t)
	attachment := &AssumptionAttachment{}
	assignmentMust(t, s.AddClause(nil, AttachedAssumption(attachment)))
	clause := s.clauses.originals[len(s.clauses.originals)-1]
	var got []Assumption
	assignmentMust(t, s.assumptions.linearize(clause.Assumptions(), &got))
	if len(got) != 1 || got[0].Attachment() != attachment {
		t.Fatal("empty clause assumption", got)
	}
}
