package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPowerSumsUsesLinearDomainSigns(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	for v := uint32(0); v < 2; v++ {
		p := fmEqualityPolynomial(t, s, v, 1, 0)
		l, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{p}, []bool{false})
		assignmentMust(t, err)
		assignmentMust(t, s.AddClause([]sat.Literal{l}, Assumption{}))
	}
	positive := simpleSumLit(t, s, AtomGT, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, {{Var: 1, Degree: 1}}})
	assignmentMust(t, s.AddClause([]sat.Literal{positive.Negated()}, Assumption{}))
	ok, _, err := s.SimpleCheckPowerSums()
	assignmentMust(t, err)
	if ok {
		t.Fatal("positive linear sum constrained nonpositive")
	}
}
