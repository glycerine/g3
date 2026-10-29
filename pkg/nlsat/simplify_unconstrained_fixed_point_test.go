package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSimplifyUnconstrainedEqualitiesRecomputesOccurrencesToFixedPoint(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	xMinusY := solverPolynomial(t, s, []int32{1, -1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, {{Var: 1, Degree: 1}}})
	yMinusTwo := solverPolynomial(t, s, []int32{1, -2}, [][]polynomial.MonomialPower{{{Var: 1, Degree: 1}}, nil})
	solverUnit(t, s, AtomEQ, xMinusY)
	solverUnit(t, s, AtomEQ, yMinusTwo)
	count, supported, err := s.SimplifyUnconstrainedEqualities()
	assignmentMust(t, err)
	if !supported || count != 2 || len(s.patches) != 2 {
		t.Fatal("fixed-point elimination", count, supported, len(s.patches))
	}
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.True {
		t.Fatal(result)
	}
	for _, equality := range []polynomial.Polynomial{xMinusY, yMinusTwo} {
		sign, cause := s.config.Algebraic.EvalSign(s.config.Polynomial, equality, s.assignment)
		assignmentMust(t, cause)
		if sign != 0 {
			t.Fatal("reverse patch order violated equality", sign)
		}
	}
}
