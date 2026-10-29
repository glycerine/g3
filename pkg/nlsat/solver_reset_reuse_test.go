package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSolverResetRetainsVariablesStatisticsAndReusesSolver(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	plain, err := s.MkBoolVar()
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	positive, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	negative, err := s.MkInequality(AtomLT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{positive}, Assumption{}))
	assignmentMust(t, s.AddClause([]sat.Literal{negative}, Assumption{}))
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.False {
		t.Fatal("fixture result", result)
	}
	statistics, err := s.Statistics()
	assignmentMust(t, err)
	assignmentMust(t, s.Reset())
	after, err := s.Statistics()
	assignmentMust(t, err)
	if after["nlsat conflicts"] != statistics["nlsat conflicts"] || after["nlsat stages"] != statistics["nlsat stages"] {
		t.Fatal("reset changed cumulative statistics", statistics, after)
	}
	if len(s.clauses.originals) != 0 || len(s.clauses.learned) != 0 || len(s.assignment.assigned) != 0 {
		t.Fatal("reset state", len(s.clauses.originals), len(s.clauses.learned), len(s.assignment.assigned))
	}
	if s.atoms.bools.dead[plain] {
		t.Fatal("plain Boolean variable was reclaimed")
	}
	n, err := s.NumVariables()
	assignmentMust(t, err)
	if n != 1 {
		t.Fatal("arithmetic variables not retained", n)
	}
	result, err = s.Check()
	assignmentMust(t, err)
	if result != lbool.True {
		t.Fatal("empty reused solver", result)
	}
	assignmentMust(t, s.AddClause([]sat.Literal{sat.MakeLiteral(plain, false)}, Assumption{}))
	result, err = s.Check()
	assignmentMust(t, err)
	if result != lbool.True {
		t.Fatal("retained plain variable", result)
	}
}
