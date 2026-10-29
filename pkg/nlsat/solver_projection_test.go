package nlsat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSolverProjectOwnsResultAndRestoresVariableOrder(t *testing.T) {
	s := solverFixtureConfig(t, SolverConfig{SimplifyCore: true, Factor: true})
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	// x1 - x0 > 0 is eliminated; x1 > 0 is independent of x0 and must
	// survive. Eliminating x0 also exercises the temporary x0/x1 reorder.
	dependent := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{-1, 1, 0}, evaluatorTerm{1, 0, 1})
	independent := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 0, 1})
	p, err := s.MkInequality(AtomGT, []polynomial.Polynomial{dependent}, []bool{false}, false)
	assignmentMust(t, err)
	q, err := s.MkInequality(AtomGT, []polynomial.Polynomial{independent}, []bool{false}, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{p}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{q}, Assumption{}))
	value, err := s.Check()
	assignmentMust(t, err)
	if value != lbool.True {
		t.Fatal("projection fixture is not satisfiable", value)
	}

	result, err := NewLiteralVector(s)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, result.Close()) })
	assignmentMust(t, s.Project(0, []sat.Literal{p, q}, result))
	var literals []sat.Literal
	assignmentMust(t, result.Literals(&literals))
	if len(literals) != 1 || literals[0] != q {
		t.Fatal("wrong projected clause", literals, q)
	}
	for variable := range s.clauses.vars.perm {
		if s.clauses.vars.perm[variable] != uint32(variable) || s.clauses.vars.inv[variable] != uint32(variable) {
			t.Fatal("projection did not restore variable order", s.clauses.vars.perm, s.clauses.vars.inv)
		}
	}
	if got, err := s.LiteralValue(literals[0]); err != nil || got != lbool.True {
		t.Fatal("projected literal is not true in the current model", got, err)
	}
}

func TestLiteralVectorBlocksSolverCloseUntilReleased(t *testing.T) {
	s := solverFixture(t)
	v, err := NewLiteralVector(s)
	assignmentMust(t, err)
	if err = s.Close(); !errors.Is(err, ErrSearchPending) {
		t.Fatal("live literal vector did not protect its solver", err)
	}
	assignmentMust(t, v.Close())
	assignmentMust(t, s.Close())
}
