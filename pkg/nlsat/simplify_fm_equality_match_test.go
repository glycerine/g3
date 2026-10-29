package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyFourierMotzkinMatchingBoundsSubstituteEquality(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	} // y,x
	p := fmEqualityPolynomial(t, s, 1, 1, -2)
	lt, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	gt, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{lt.Negated()}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{gt.Negated()}, Assumption{}))
	xy := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 1, Degree: 1}}, {{Var: 0, Degree: 1}}})
	solverUnit(t, s, AtomGT, xy)
	fmBlockVariable(t, s, 0)
	count, _, err := s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	if count != 1 || len(s.fmBounds) != 2 {
		t.Fatal(count, len(s.fmBounds))
	}
}
