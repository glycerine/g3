package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func simpleSumLit(t *testing.T, s *Solver, kind AtomKind, coeff []int32, powers [][]polynomial.MonomialPower) sat.Literal {
	t.Helper()
	p := solverPolynomial(t, s, coeff, powers)
	l, err := s.MkInequalityLiteral(kind, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	return l
}
func TestSimpleCheckPowerSumsRejectsNegativeSumOfSquares(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	l := simpleSumLit(t, s, AtomLT, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 2}}, {{Var: 1, Degree: 2}}})
	assignmentMust(t, s.AddClause([]sat.Literal{l}, Assumption{}))
	ok, _, err := s.SimpleCheckPowerSums()
	assignmentMust(t, err)
	if ok {
		t.Fatal("negative sum of squares accepted")
	}
}
