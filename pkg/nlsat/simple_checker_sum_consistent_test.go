package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPowerSumsAcceptsNonnegativeSquares(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	lt := simpleSumLit(t, s, AtomLT, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 2}}, {{Var: 1, Degree: 2}}})
	assignmentMust(t, s.AddClause([]sat.Literal{lt.Negated()}, Assumption{}))
	ok, learned, err := s.SimpleCheckPowerSums()
	assignmentMust(t, err)
	if !ok || len(learned) != 0 {
		t.Fatal(ok, learned)
	}
}
