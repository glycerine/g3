package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPowerSumsUsesPositiveConstant(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	gt := simpleSumLit(t, s, AtomGT, []int32{1, 1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 2}}, {{Var: 1, Degree: 2}}, nil})
	le := gt.Negated()
	assignmentMust(t, s.AddClause([]sat.Literal{le}, Assumption{}))
	ok, _, err := s.SimpleCheckPowerSums()
	assignmentMust(t, err)
	if ok {
		t.Fatal("positive sum constrained nonpositive")
	}
}
