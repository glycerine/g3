package nlsat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func TestSolverHostVariableAndAtomViewsAreSnapshots(t *testing.T) {
	s := solverFixture(t)
	realVariable, err := s.MkVar(false)
	assignmentMust(t, err)
	integerVariable, err := s.MkVar(true)
	assignmentMust(t, err)
	if realVariable != 0 || integerVariable != 1 {
		t.Fatal("variable sequence", realVariable, integerVariable)
	}
	count, err := s.NumVariables()
	assignmentMust(t, err)
	if count != 2 {
		t.Fatal("variable count", count)
	}
	integer, err := s.IsInteger(0)
	assignmentMust(t, err)
	if integer {
		t.Fatal("real variable reported integer")
	}
	integer, err = s.IsInteger(1)
	assignmentMust(t, err)
	if !integer {
		t.Fatal("integer variable reported real")
	}
	if _, err = s.IsInteger(2); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("out-of-range integer query", err)
	}
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	literal, err := s.MkInequality(AtomEQ, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	var atoms []ArithmeticAtom
	assignmentMust(t, s.AtomSnapshot(&atoms))
	if atoms[literal.Variable()] == nil {
		t.Fatal("atom snapshot omitted arithmetic atom")
	}
	atoms[literal.Variable()] = nil
	if s.atoms.bools.atoms[literal.Variable()] == nil {
		t.Fatal("atom snapshot aliased solver vector")
	}
	s.clauses.vars.equalities[0] = s.atoms.bools.atoms[literal.Variable()]
	var equalities []ArithmeticAtom
	assignmentMust(t, s.VariableEqualitySnapshot(&equalities))
	if len(equalities) != 2 || equalities[0] == nil || equalities[1] != nil {
		t.Fatal("variable-equality snapshot", equalities)
	}
	equalities[0] = nil
	if s.clauses.vars.equalities[0] == nil {
		t.Fatal("equality snapshot aliased solver vector")
	}
	s.clauses.vars.equalities[0] = nil
	evaluator, err := s.Evaluator()
	assignmentMust(t, err)
	if evaluator != s.evaluator {
		t.Fatal("wrong borrowed evaluator")
	}
	truth, err := s.True()
	assignmentMust(t, err)
	if truth != TrueLiteral {
		t.Fatal("true literal", truth)
	}
}
