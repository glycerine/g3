package nlsat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSolverAtomVariableAndDegreeQueries(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	plain, err := s.MkBoolVar()
	assignmentMust(t, err)
	max, err := s.BooleanVariableMaxVariable(plain)
	assignmentMust(t, err)
	if max != NullVariable {
		t.Fatal("plain Boolean maximum", max)
	}
	degree, err := s.LiteralDegree(sat.MakeLiteral(plain, false))
	assignmentMust(t, err)
	if degree != 0 {
		t.Fatal("plain Boolean degree", degree)
	}
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 0, 3}, evaluatorTerm{1, 1, 0})
	literal, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	max, err = s.LiteralMaxVariable(literal)
	assignmentMust(t, err)
	if max != 1 {
		t.Fatal("arithmetic literal maximum", max)
	}
	degree, err = s.LiteralDegree(literal)
	assignmentMust(t, err)
	if degree != 3 {
		t.Fatal("arithmetic literal degree", degree)
	}
	atom, found, err := s.AtomForBooleanVariable(literal.Variable())
	assignmentMust(t, err)
	if !found {
		t.Fatal("arithmetic atom missing")
	}
	degree, err = s.ArithmeticAtomDegree(atom)
	assignmentMust(t, err)
	if degree != 3 {
		t.Fatal("arithmetic atom degree", degree)
	}
	foreign := &InequalityAtom{}
	if err = foreign.initialize(AtomGT, []polynomial.Polynomial{p}, []bool{false}, 1, make([]InequalityFactor, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ArithmeticAtomDegree(foreign); !errors.Is(err, ErrAtomCacheOwner) {
		t.Fatal("foreign atom degree accepted", err)
	}
}
