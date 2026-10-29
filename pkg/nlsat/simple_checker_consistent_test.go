package nlsat

import (
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckLinearBoundsAcceptsSingleton(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	le := simpleLinearLit(t, s, AtomGT, 1, 0).Negated()
	ge := simpleLinearLit(t, s, AtomLT, 1, 0).Negated()
	assignmentMust(t, s.AddClause([]sat.Literal{le}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{ge}, Assumption{}))
	ok, learned, err := s.SimpleCheckLinearBounds()
	assignmentMust(t, err)
	if !ok || len(learned) != 0 {
		t.Fatal(ok, learned)
	}
}
