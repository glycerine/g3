package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func simplePowerLit(t *testing.T, s *Solver, variable, degree uint32, kind AtomKind, a, c int32) sat.Literal {
	t.Helper()
	p := solverPolynomial(t, s, []int32{a, c}, [][]polynomial.MonomialPower{{{Var: variable, Degree: degree}}, nil})
	l, err := s.MkInequalityLiteral(kind, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	return l
}
func TestSimpleCheckPowerBoundsOddRootConflict(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	lt := simplePowerLit(t, s, 0, 3, AtomLT, 1, 0)
	assignmentMust(t, s.AddClause([]sat.Literal{lt}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{lt.Negated()}, Assumption{}))
	ok, _, err := s.SimpleCheckPowerBounds()
	assignmentMust(t, err)
	if ok {
		t.Fatal("odd conflict accepted")
	}
}
