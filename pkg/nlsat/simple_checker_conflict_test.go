package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func simpleLinearLit(t *testing.T, s *Solver, kind AtomKind, a, c int32) sat.Literal {
	t.Helper()
	p := fmEqualityPolynomial(t, s, 0, a, c)
	l, err := s.MkInequalityLiteral(kind, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	return l
}
func TestSimpleCheckLinearBoundsFindsClosedOpenConflict(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	lt := simpleLinearLit(t, s, AtomLT, 1, 0)
	ge := simpleLinearLit(t, s, AtomLT, 1, 0).Negated()
	assignmentMust(t, s.AddClause([]sat.Literal{lt}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{ge}, Assumption{}))
	ok, _, err := s.SimpleCheckLinearBounds()
	assignmentMust(t, err)
	if ok {
		t.Fatal("contradiction accepted")
	}
}
