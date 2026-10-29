package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSimplifyUnconstrainedEqualityRestoresRealModel(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	equality := solverPolynomial(t, s, []int32{1, 2}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
	solverUnit(t, s, AtomEQ, equality)
	count, supported, err := s.SimplifyUnconstrainedEqualities()
	assignmentMust(t, err)
	if !supported || count != 1 || len(s.patches) != 1 || len(s.clauses.originals) != 1 {
		t.Fatal("unconstrained real elimination", count, supported, len(s.patches), len(s.clauses.originals))
	}
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.True {
		t.Fatal("eliminated real check", result)
	}
	sign, err := s.config.Algebraic.EvalSign(s.config.Polynomial, equality, s.assignment)
	assignmentMust(t, err)
	if sign != 0 {
		t.Fatal("restored model violates eliminated equality", sign)
	}
}
