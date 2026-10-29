package nlsat

import (
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckLinearBoundsRepeatedOwnership(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	l := simpleLinearLit(t, s, AtomGT, 1, -2)
	assignmentMust(t, s.AddClause([]sat.Literal{l}, Assumption{}))
	for i := 0; i < 3; i++ {
		ok, _, cause := s.SimpleCheckLinearBounds()
		assignmentMust(t, cause)
		if !ok {
			t.Fatal(i)
		}
	}
}
