package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifySubsumedClausesPublishesUnitResolvent(t *testing.T) {
	s, one, squarePlusOne, oddPlusOne := subsumptionPolynomialFixture(t)
	unit, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{squarePlusOne}, []bool{false})
	assignmentMust(t, err)
	oppositeWeak, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{one}, []bool{false})
	assignmentMust(t, err)
	remaining, err := s.MkInequalityLiteral(AtomEQ, []polynomial.Polynomial{oddPlusOne}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{unit}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{oppositeWeak.Negated(), remaining}, Assumption{}))
	target := s.clauses.originals[len(s.clauses.originals)-1]
	count, err := s.SimplifySubsumedClauses()
	assignmentMust(t, err)
	if count != 1 || s.clauses.owned[target] != nil || len(s.clauses.originals) != 3 {
		t.Fatal("unit subsumption lifecycle", count, len(s.clauses.originals))
	}
	resolvent := s.clauses.originals[len(s.clauses.originals)-1]
	if resolvent.Size() != 1 || resolvent.Literals()[0] != remaining {
		t.Fatal("unit resolvent", resolvent.Literals(), remaining)
	}
}
