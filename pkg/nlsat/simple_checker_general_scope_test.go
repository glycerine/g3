package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPolynomialSignsLeavesUnknownProductPossible(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	p := solverPolynomial(t, s, []int32{1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}, {Var: 1, Degree: 1}}})
	l, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{l}, Assumption{}))
	ok, learned, err := s.SimpleCheckPolynomialSigns()
	assignmentMust(t, err)
	if !ok || len(learned) != 0 {
		t.Fatal(ok, learned)
	}
}
