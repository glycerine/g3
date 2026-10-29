package nlsat

import (
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPowerBoundsNegativeEvenThreshold(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	impossible := simplePowerLit(t, s, 0, 2, AtomLT, 1, 1)
	assignmentMust(t, s.AddClause([]sat.Literal{impossible}, Assumption{}))
	ok, _, err := s.SimpleCheckPowerBounds()
	assignmentMust(t, err)
	if ok {
		t.Fatal("square below negative threshold accepted")
	}
}
func TestSimpleCheckPowerBoundsEvenAlwaysPositive(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	positive := simplePowerLit(t, s, 0, 2, AtomGT, 1, 1)
	assignmentMust(t, s.AddClause([]sat.Literal{positive}, Assumption{}))
	ok, _, err := s.SimpleCheckPowerBounds()
	assignmentMust(t, err)
	if !ok {
		t.Fatal("positive square rejected")
	}
}
