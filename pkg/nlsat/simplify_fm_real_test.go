package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func fmIntervalFixture(t *testing.T, integer bool, lower, upper int32) (*Solver, polynomial.Polynomial, polynomial.Polynomial) {
	t.Helper()
	s := solverFixture(t)
	_, err := s.MkVar(integer)
	assignmentMust(t, err)
	lo := solverPolynomial(t, s, []int32{1, -lower}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
	hi := solverPolynomial(t, s, []int32{1, -upper}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
	solverUnit(t, s, AtomGT, lo)
	solverUnit(t, s, AtomLT, hi)
	return s, lo, hi
}

func TestSimplifyFourierMotzkinRestoresStrictRealIntervalModel(t *testing.T) {
	s, lower, upper := fmIntervalFixture(t, false, 1, 3)
	count, supported, err := s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	if !supported || count != 1 || len(s.fmBounds) != 2 || len(s.clauses.originals) != 1 {
		t.Fatal("elimination", count, supported, len(s.fmBounds), len(s.clauses.originals))
	}
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.True {
		t.Fatal("check", result)
	}
	loSign, err := s.config.Algebraic.EvalSign(s.config.Polynomial, lower, s.assignment)
	assignmentMust(t, err)
	hiSign, err := s.config.Algebraic.EvalSign(s.config.Polynomial, upper, s.assignment)
	assignmentMust(t, err)
	if loSign <= 0 || hiSign >= 0 {
		t.Fatal("patched interval", loSign, hiSign)
	}
}
