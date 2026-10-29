package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPolynomialSignsMultipliesFactorSigns(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	simplePositiveUnits(t, s, 2)
	x := fmEqualityPolynomial(t, s, 0, 1, 0)
	y := fmEqualityPolynomial(t, s, 1, 1, 0)
	l, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{x, y}, []bool{false, false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{l}, Assumption{}))
	ok, _, err := s.SimpleCheckPolynomialSigns()
	assignmentMust(t, err)
	if ok {
		t.Fatal("positive factor product accepted negative")
	}
}
