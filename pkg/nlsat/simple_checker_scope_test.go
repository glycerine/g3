package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckLinearBoundsLeavesQuadraticUnknown(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 2}}, nil})
	l, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{l}, Assumption{}))
	ok, learned, err := s.SimpleCheckLinearBounds()
	assignmentMust(t, err)
	if !ok || len(learned) != 0 {
		t.Fatal(ok, learned)
	}
}
