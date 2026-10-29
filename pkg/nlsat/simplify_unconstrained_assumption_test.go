package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyUnconstrainedEqualityRetiresClauseAssumption(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := solverPolynomial(t, s, []int32{1, -4}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
	literal, err := s.MkInequality(AtomEQ, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	attachment := &AssumptionAttachment{}
	assignmentMust(t, s.AddClause([]sat.Literal{literal}, AttachedAssumption(attachment)))
	count, supported, err := s.SimplifyUnconstrainedEqualities()
	assignmentMust(t, err)
	if !supported || count != 1 || len(s.assumptions.live) != 0 {
		t.Fatal("eliminated clause retained assumption", count, supported, len(s.assumptions.live))
	}
	statistics, err := s.Statistics()
	assignmentMust(t, err)
	if statistics["nlsat simplifications"] != 1 {
		t.Fatal(statistics)
	}
}
