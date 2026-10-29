package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func simplePositiveUnits(t *testing.T, s *Solver, n uint32) {
	t.Helper()
	for v := uint32(0); v < n; v++ {
		p := fmEqualityPolynomial(t, s, v, 1, 0)
		l, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{p}, []bool{false})
		assignmentMust(t, err)
		assignmentMust(t, s.AddClause([]sat.Literal{l}, Assumption{}))
	}
}
func TestSimpleCheckPolynomialSignsMixedMonomialConflict(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	simplePositiveUnits(t, s, 2)
	p := solverPolynomial(t, s, []int32{1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}, {Var: 1, Degree: 1}}})
	l, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{l}, Assumption{}))
	ok, _, err := s.SimpleCheckPolynomialSigns()
	assignmentMust(t, err)
	if ok {
		t.Fatal("positive mixed monomial accepted negative")
	}
}
