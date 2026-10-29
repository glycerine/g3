package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSimplifyFourierMotzkinEqualityZeroFactorBecomesFalse(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	} // y,x
	x := fmEqualityPolynomial(t, s, 1, 1, 0)
	y := fmEqualityPolynomial(t, s, 0, 1, 0)
	solverUnit(t, s, AtomEQ, x)
	product, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{x, y}, []bool{false, false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{product}, Assumption{}))
	count, _, err := s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	if count != 1 {
		t.Fatal("elimination", count)
	}
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.False {
		t.Fatal("zero factor result", result)
	}
}
