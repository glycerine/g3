package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPowerSumsLearnsImpossibleDisjunct(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 3; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	bad := simpleSumLit(t, s, AtomLT, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 2}}, {{Var: 1, Degree: 2}}})
	good := simplePowerLit(t, s, 2, 2, AtomGT, 1, 0)
	assignmentMust(t, s.AddClause([]sat.Literal{bad, good}, Assumption{}))
	ok, learned, err := s.SimpleCheckPowerSums()
	assignmentMust(t, err)
	found := false
	for _, l := range learned {
		found = found || l == bad.Negated()
	}
	if !ok || !found {
		t.Fatal(ok, learned)
	}
}
