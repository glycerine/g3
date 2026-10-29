package nlsat

import "testing"

func TestSimplifyDriverFactorRewriteRetainsAssumption(t *testing.T) {
	attachment := &AssumptionAttachment{}
	s, _, _, _ := factorSimplificationFixture(t, AtomEQ, false, [2]bool{}, AttachedAssumption(attachment))
	fmBlockVariable(t, s, 0)
	fmBlockVariable(t, s, 1)
	// Force the source size-decrease gate so factor splitting is scheduled.
	last := s.clauses.originals[len(s.clauses.originals)-1]
	assignmentMust(t, s.AddClause(last.Literals(), AttachedAssumption(attachment)))
	_, err := s.Simplify()
	assignmentMust(t, err)
	found := false
	for _, clause := range s.clauses.originals {
		var values []Assumption
		assignmentMust(t, s.assumptions.linearize(clause.Assumptions(), &values))
		for _, value := range values {
			found = found || value.Attachment() == attachment
		}
	}
	if !found {
		t.Fatal("factor assumption lost")
	}
}
