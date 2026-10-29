package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifySubsumedClausesRemovesArithmeticWeakerClause(t *testing.T) {
	s, one, squarePlusOne, _ := subsumptionPolynomialFixture(t)
	strong, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{squarePlusOne}, []bool{false})
	assignmentMust(t, err)
	weak, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{one}, []bool{false})
	assignmentMust(t, err)
	extra, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{squarePlusOne}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{strong}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{weak, extra}, Assumption{}))
	target := s.clauses.originals[len(s.clauses.originals)-1]
	count, err := s.SimplifySubsumedClauses()
	assignmentMust(t, err)
	if count != 1 || s.clauses.owned[target] != nil || len(s.clauses.originals) != 2 {
		t.Fatal("subsumed clause lifetime", count, len(s.clauses.originals), s.clauses.owned[target])
	}
}
