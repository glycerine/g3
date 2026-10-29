package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSolverResetClearsModelTrailLearnedClausesAndCore(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	positive, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	negative, err := s.MkInequality(AtomLT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	attachment := &AssumptionAttachment{}
	assignmentMust(t, s.AddClause([]sat.Literal{positive}, AttachedAssumption(attachment)))
	assignmentMust(t, s.AddClause([]sat.Literal{negative}, AttachedAssumption(attachment)))
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.False {
		t.Fatal("fixture conflict", result)
	}
	var core []Assumption
	assignmentMust(t, s.Core(&core))
	if len(core) == 0 {
		t.Fatal("fixture core")
	}
	assignmentMust(t, s.Reset())
	assignmentMust(t, s.Core(&core))
	if len(core) != 0 || len(s.search.trail) != 0 || s.search.level != 0 || s.search.xk != NullVariable || len(s.clauses.learned) != 0 {
		t.Fatal("reset search/core", core, len(s.search.trail), s.search.level, s.search.xk, len(s.clauses.learned))
	}
	assigned, err := s.assignment.Contains(0)
	assignmentMust(t, err)
	if assigned {
		t.Fatal("model assignment survived")
	}
}
