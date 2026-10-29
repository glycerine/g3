package nlsat

import (
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckLinearBoundsFlipsNegativeCoefficient(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	below := simpleLinearLit(t, s, AtomGT, -1, 1)
	atLeast := simpleLinearLit(t, s, AtomLT, 1, -1).Negated()
	assignmentMust(t, s.AddClause([]sat.Literal{below}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{atLeast}, Assumption{}))
	ok, _, err := s.SimpleCheckLinearBounds()
	assignmentMust(t, err)
	if ok {
		t.Fatal("negative coefficient conflict accepted")
	}
}
