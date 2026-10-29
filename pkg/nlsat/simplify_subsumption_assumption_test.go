package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifySubsumptionResolventJoinsBothAssumptions(t *testing.T) {
	s, one, squarePlusOne, oddPlusOne := subsumptionPolynomialFixture(t)
	unit, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{squarePlusOne}, []bool{false})
	assignmentMust(t, err)
	opposite, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{one}, []bool{false})
	assignmentMust(t, err)
	remaining, err := s.MkInequalityLiteral(AtomEQ, []polynomial.Polynomial{oddPlusOne}, []bool{false})
	assignmentMust(t, err)
	first, second := &AssumptionAttachment{}, &AssumptionAttachment{}
	assignmentMust(t, s.AddClause([]sat.Literal{unit}, AttachedAssumption(first)))
	assignmentMust(t, s.AddClause([]sat.Literal{opposite.Negated(), remaining}, AttachedAssumption(second)))
	_, err = s.SimplifySubsumedClauses()
	assignmentMust(t, err)
	resolvent := s.clauses.originals[len(s.clauses.originals)-1]
	var values []Assumption
	assignmentMust(t, s.assumptions.linearize(resolvent.Assumptions(), &values))
	found := map[*AssumptionAttachment]bool{}
	for _, value := range values {
		found[value.Attachment()] = true
	}
	if len(values) != 2 || !found[first] || !found[second] {
		t.Fatal("joined assumptions", values)
	}
}
