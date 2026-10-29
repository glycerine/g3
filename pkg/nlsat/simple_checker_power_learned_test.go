package nlsat

import (
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPowerBoundsLearnsImpossibleDisjunct(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	bad := simplePowerLit(t, s, 0, 2, AtomLT, 1, 0)
	good := simplePowerLit(t, s, 1, 2, AtomGT, 1, 0)
	assignmentMust(t, s.AddClause([]sat.Literal{bad, good}, Assumption{}))
	ok, learned, err := s.SimpleCheckPowerBounds()
	assignmentMust(t, err)
	found := false
	for _, l := range learned {
		found = found || l == bad.Negated()
	}
	if !ok || !found {
		t.Fatal(ok, learned)
	}
}
