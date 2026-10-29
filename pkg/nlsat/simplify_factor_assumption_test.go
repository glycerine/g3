package nlsat

import "testing"

func TestSimplifyUnitFactorsPreservesAssumptionsAndStatistics(t *testing.T) {
	attachment := &AssumptionAttachment{}
	s, p, q, old := factorSimplificationFixture(t, AtomLT, false, [2]bool{}, AttachedAssumption(attachment))
	oldAssumption := old.Assumptions()
	count, err := s.SimplifyUnitFactors()
	assignmentMust(t, err)
	if count != 1 || s.clauses.owned[old] != nil {
		t.Fatal("source clause was not retired", count)
	}
	factorWant(t, factorClauseSignatures(t, s, p, q), []string{"p:gt,q:gt", "p:lt,q:lt"})
	for _, clause := range s.clauses.originals {
		if clause.Literals()[0] == TrueLiteral {
			continue
		}
		if clause.Assumptions() != oldAssumption {
			t.Fatal("replacement did not share source assumption DAG", clause.Assumptions(), oldAssumption)
		}
		var values []Assumption
		assignmentMust(t, s.assumptions.linearize(clause.Assumptions(), &values))
		if len(values) != 1 || values[0].Attachment() != attachment {
			t.Fatal("replacement assumption payload", values)
		}
	}
	statistics, err := s.Statistics()
	assignmentMust(t, err)
	if statistics["nlsat simplifications"] != 1 {
		t.Fatal("simplification statistic", statistics)
	}
	assignmentMust(t, s.ResetStatistics())
	statistics, err = s.Statistics()
	assignmentMust(t, err)
	if statistics["nlsat simplifications"] != 0 {
		t.Fatal("simplification statistic reset", statistics)
	}
}
