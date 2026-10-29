package nlsat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func TestSolverResetHonorsScopedLiteralVectorReferences(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	literal, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	vector, err := NewLiteralVector(s)
	assignmentMust(t, err)
	assignmentMust(t, vector.Push(literal))
	if err = s.Reset(); !errors.Is(err, ErrAtomCacheLive) {
		t.Fatal("vector reference reset", err)
	}
	assignmentMust(t, vector.Reset())
	assignmentMust(t, s.Reset())
	assignmentMust(t, vector.Close())
	if !s.atoms.bools.dead[literal.Variable()] {
		t.Fatal("atom survived cleared vector reset")
	}
}
