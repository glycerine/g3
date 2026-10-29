package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func fmEqualityPolynomial(t *testing.T, s *Solver, variable uint32, coefficient, constant int32) polynomial.Polynomial {
	t.Helper()
	return solverPolynomial(t, s, []int32{coefficient, constant}, [][]polynomial.MonomialPower{{{Var: variable, Degree: 1}}, nil})
}

func TestSimplifyFourierMotzkinEqualityRestoresExactModel(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	equality := fmEqualityPolynomial(t, s, 0, 1, -2)
	solverUnit(t, s, AtomEQ, equality)
	count, supported, err := s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	if !supported || count != 1 || len(s.fmBounds) != 2 || len(s.clauses.originals) != 1 {
		t.Fatal(count, supported, len(s.fmBounds), len(s.clauses.originals))
	}
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.True {
		t.Fatal(result)
	}
	sign, err := s.config.Algebraic.EvalSign(s.config.Polynomial, equality, s.assignment)
	assignmentMust(t, err)
	if sign != 0 {
		t.Fatal("equality patch", sign)
	}
}
