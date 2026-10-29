package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func TestSimplifyUnconstrainedIntegerRequiresUnitCoefficient(t *testing.T) {
	for _, test := range []struct {
		name        string
		coefficient int32
		want        uint32
	}{
		{"unit", -1, 1},
		{"non-unit", 2, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := solverFixture(t)
			_, err := s.MkVar(true)
			assignmentMust(t, err)
			p := solverPolynomial(t, s, []int32{test.coefficient, 3}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
			solverUnit(t, s, AtomEQ, p)
			count, supported, err := s.SimplifyUnconstrainedEqualities()
			assignmentMust(t, err)
			if !supported || count != test.want || uint32(len(s.patches)) != test.want {
				t.Fatal("integer coefficient policy", count, supported, len(s.patches), test.want)
			}
		})
	}
}
