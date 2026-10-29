package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSubsumptionLiteralUsesArithmeticImplicationMatrix(t *testing.T) {
	s, one, squarePlusOne, _ := subsumptionPolynomialFixture(t)
	pLT, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{squarePlusOne}, []bool{false})
	assignmentMust(t, err)
	qLT, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{one}, []bool{false})
	assignmentMust(t, err)
	qGT, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{one}, []bool{false})
	assignmentMust(t, err)
	phase := &subsumptionSimplifier{solver: s}
	for _, test := range []struct {
		left, right sat.Literal
		want        bool
	}{
		{pLT, qLT, true},
		{pLT, qGT.Negated(), true},
		{qLT, pLT, false},
		{pLT, qGT, false},
	} {
		got, cause := phase.literalSubsumes(test.left, test.right)
		assignmentMust(t, cause)
		if got != test.want {
			t.Fatal("literal implication", test.left, test.right, got, test.want)
		}
	}
}
