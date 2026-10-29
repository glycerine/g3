package nlsat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSolverMaximizeSelectsLeastUpperRootAndIgnoresIndependentLiterals(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	evaluatorSetFraction(t, s.evaluator, 0, 1, 1)
	evaluatorSetFraction(t, s.evaluator, 1, 7, 1)
	quadratic := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 2, 0}, evaluatorTerm{-4, 0, 0})
	linear := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0}, evaluatorTerm{-5, 0, 0})
	independent := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 0, 1}, evaluatorTerm{-9, 0, 0})
	literals := make([]sat.Literal, 0, 3)
	for _, p := range []polynomial.Polynomial{linear, independent, quadratic} {
		literal, err := s.MkInequality(AtomEQ, []polynomial.Polynomial{p}, []bool{false}, false)
		assignmentMust(t, err)
		literals = append(literals, literal)
	}
	out := assignmentInteger(t, s.config.Algebraic, 99)
	unbounded, err := s.Maximize(0, literals, out)
	assignmentMust(t, err)
	if unbounded {
		t.Fatal("upper root was reported unbounded")
	}
	assignmentWantInteger(t, s.config.Algebraic, out, 2)
}

func TestSolverMaximizeUnboundedPreservesOutput(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	evaluatorSetFraction(t, s.evaluator, 0, 6, 1)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 2, 0}, evaluatorTerm{-4, 0, 0})
	literal, err := s.MkInequality(AtomEQ, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	out := assignmentInteger(t, s.config.Algebraic, 99)
	unbounded, err := s.Maximize(0, []sat.Literal{literal}, out)
	assignmentMust(t, err)
	if !unbounded {
		t.Fatal("root below current value bounded maximization")
	}
	assignmentWantInteger(t, s.config.Algebraic, out, 99)
}

func TestSolverMaximizeValidationPrefixes(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	out := assignmentInteger(t, s.config.Algebraic, 17)
	if _, err = s.Maximize(1, nil, out); !errors.Is(err, arena.ErrBounds) {
		t.Fatalf("variable bound error = %v", err)
	}
	if _, err = s.Maximize(0, nil, out); !errors.Is(err, arena.ErrConfig) {
		t.Fatalf("unassigned variable error = %v", err)
	}
	if _, err = s.Maximize(0, nil, nil); err == nil {
		t.Fatal("nil output accepted")
	}
	assignmentWantInteger(t, s.config.Algebraic, out, 17)
}
