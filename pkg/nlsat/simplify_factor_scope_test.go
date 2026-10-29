package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyUnitFactorsUsesOriginalUnitTwoFactorPrefixOnly(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	q := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 0, 1})
	one, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	product, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p, q}, []bool{false, false}, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{one, product}, Assumption{})) // non-unit
	_, err = s.clauses.mkClause([]sat.Literal{product}, true, nil)            // learned
	assignmentMust(t, err)
	count, err := s.SimplifyUnitFactors()
	assignmentMust(t, err)
	if count != 0 || len(s.clauses.originals) != 2 || len(s.clauses.learned) != 1 {
		t.Fatal("out-of-scope clauses changed", count, len(s.clauses.originals), len(s.clauses.learned))
	}
}
