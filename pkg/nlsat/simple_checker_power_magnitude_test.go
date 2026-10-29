package nlsat

import (
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPowerBoundsMagnitudeEqualityConflict(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	eq := simplePowerLit(t, s, 0, 2, AtomEQ, 1, -4)
	gt := simplePowerLit(t, s, 0, 2, AtomGT, 1, -9)
	assignmentMust(t, s.AddClause([]sat.Literal{eq}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{gt}, Assumption{}))
	ok, _, err := s.SimpleCheckPowerBounds()
	assignmentMust(t, err)
	if ok {
		t.Fatal("incompatible magnitudes accepted")
	}
}
