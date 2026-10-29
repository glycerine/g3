package nlsat

import (
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPowerBoundsOpenClosedBoundary(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	lt := simplePowerLit(t, s, 0, 2, AtomLT, 1, -4)
	ge := lt.Negated()
	assignmentMust(t, s.AddClause([]sat.Literal{lt}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{ge}, Assumption{}))
	ok, _, err := s.SimpleCheckPowerBounds()
	assignmentMust(t, err)
	if ok {
		t.Fatal("boundary conflict accepted")
	}
}
