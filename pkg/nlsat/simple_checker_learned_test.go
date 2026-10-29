package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckLinearBoundsLearnsImpossibleDisjunct(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	xlt := simpleLinearLit(t, s, AtomLT, 1, 0)
	xge := xlt.Negated()
	assignmentMust(t, s.AddClause([]sat.Literal{xge}, Assumption{}))
	yp := fmEqualityPolynomial(t, s, 1, 1, 0)
	ygt, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{yp}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{xlt, ygt}, Assumption{}))
	ok, learned, err := s.SimpleCheckLinearBounds()
	assignmentMust(t, err)
	found := false
	for _, l := range learned {
		found = found || l == xlt.Negated()
	}
	if !ok || !found {
		t.Fatal(ok, learned)
	}
}
