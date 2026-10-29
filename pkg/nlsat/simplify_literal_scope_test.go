package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyLiteralContentPreservesPolarityAssumptionAndLearnedScope(t *testing.T) {
	s, _, positive, original := contentSimplificationFixture(t, true, AtomEQ, 6, 9)
	attachment := &AssumptionAttachment{}
	negative := positive.Negated()
	assignmentMust(t, s.AddClause([]sat.Literal{negative}, AttachedAssumption(attachment)))
	attached := s.clauses.originals[len(s.clauses.originals)-1]
	learned, err := s.clauses.mkClause([]sat.Literal{positive}, true, nil)
	assignmentMust(t, err)
	assignmentMust(t, s.clauses.deleteClause(original))
	count, err := s.SimplifyLiteralContent()
	assignmentMust(t, err)
	if count != 1 || s.clauses.owned[learned] == nil || learned.Literals()[0] != positive {
		t.Fatal("scope", count)
	}
	if s.clauses.owned[attached] != nil {
		t.Fatal("old attached clause retained")
	}
	replacement := s.clauses.originals[len(s.clauses.originals)-1]
	if !replacement.Literals()[0].Sign() || replacement.Assumptions() == nil {
		t.Fatal("polarity or assumption lost")
	}
	var assumptions []Assumption
	assignmentMust(t, s.assumptions.linearize(replacement.Assumptions(), &assumptions))
	if len(assumptions) != 1 || assumptions[0].Attachment() != attachment {
		t.Fatal("assumption", assumptions)
	}
}
