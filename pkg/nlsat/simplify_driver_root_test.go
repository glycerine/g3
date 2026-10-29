package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyDriverRootGuardLeavesEliminationPhasesInactive(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	p := solverPolynomial(t, s, []int32{1, -1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
	root, err := s.MkRoot(AtomRootEQ, 1, 1, p, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{root}, Assumption{}))
	solverUnit(t, s, AtomGT, p)
	before := len(s.clauses.originals)
	summary, err := s.Simplify()
	assignmentMust(t, err)
	if summary.Unconstrained != 0 || summary.FourierMotzkin != 0 || len(s.clauses.originals) != before {
		t.Fatal(summary, len(s.clauses.originals), before)
	}
}
