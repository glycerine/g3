package nlsat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSolverResetRefusesExternalAtomReferenceWithoutDeletingClauses(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	literal, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{literal}, Assumption{}))
	assignmentMust(t, s.RetainLiteral(literal))
	before := append([]*Clause(nil), s.clauses.originals...)
	if err = s.Reset(); !errors.Is(err, ErrAtomCacheLive) {
		t.Fatal("live atom reset", err)
	}
	if len(s.clauses.originals) != len(before) || s.clauses.originals[0] != before[0] || s.clauses.originals[1] != before[1] || s.atoms.bools.dead[literal.Variable()] {
		t.Fatal("refused reset mutated clauses/atom")
	}
	assignmentMust(t, s.ReleaseLiteral(literal))
	assignmentMust(t, s.Reset())
	if len(s.clauses.originals) != 0 || !s.atoms.bools.dead[literal.Variable()] {
		t.Fatal("released reset did not reclaim")
	}
}
