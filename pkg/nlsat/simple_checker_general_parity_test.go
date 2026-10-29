package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPolynomialSignsUsesEvenFactorParity(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	x := fmEqualityPolynomial(t, s, 0, 1, 0)
	l, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{x}, []bool{true})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{l}, Assumption{}))
	ok, _, err := s.SimpleCheckPolynomialSigns()
	assignmentMust(t, err)
	if ok {
		t.Fatal("even factor accepted negative")
	}
}
