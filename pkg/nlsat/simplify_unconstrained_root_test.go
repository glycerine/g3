package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyUnconstrainedEqualitiesGlobalRootGuard(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	equality := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
	rootPolynomial := solverPolynomial(t, s, []int32{1}, [][]polynomial.MonomialPower{{{Var: 1, Degree: 2}}})
	solverUnit(t, s, AtomEQ, equality)
	root, err := s.MkRoot(AtomRootEQ, 1, 1, rootPolynomial, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{root}, Assumption{}))
	count, supported, err := s.SimplifyUnconstrainedEqualities()
	assignmentMust(t, err)
	if supported || count != 0 || len(s.patches) != 0 || len(s.clauses.originals) != 3 {
		t.Fatal("root guard", count, supported, len(s.patches), len(s.clauses.originals))
	}
}
