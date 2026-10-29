package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func subsumptionPolynomialFixture(t *testing.T) (*Solver, polynomial.Polynomial, polynomial.Polynomial, polynomial.Polynomial) {
	t.Helper()
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	one := solverPolynomial(t, s, []int32{1}, [][]polynomial.MonomialPower{nil})
	squarePlusOne := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 2}}, nil})
	oddPlusOne := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
	return s, one, squarePlusOne, oddPlusOne
}

func TestPolynomialGreaterEqualRetainsSquareSufficientCriterion(t *testing.T) {
	s, one, squarePlusOne, oddPlusOne := subsumptionPolynomialFixture(t)
	core, err := s.config.Polynomial.ArithmeticCore()
	assignmentMust(t, err)
	for _, test := range []struct {
		left, right polynomial.Polynomial
		want        bool
	}{
		{squarePlusOne, one, true},
		{one, squarePlusOne, false},
		{oddPlusOne, one, false},
		{squarePlusOne, squarePlusOne, true},
	} {
		got, cause := core.GreaterEqual(test.left, test.right)
		assignmentMust(t, cause)
		if got != test.want {
			t.Fatal("polynomial ge criterion", got, test.want)
		}
	}
}
