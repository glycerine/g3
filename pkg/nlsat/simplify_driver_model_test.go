package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSimplifyDriverPreservesEliminatedModel(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	equality := fmEqualityPolynomial(t, s, 0, 1, -7)
	solverUnit(t, s, AtomEQ, equality)
	summary, err := s.Simplify()
	assignmentMust(t, err)
	if summary.Unconstrained != 1 {
		t.Fatal("unconstrained phase", summary)
	}
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.True {
		t.Fatal(result)
	}
	sign, err := s.config.Algebraic.EvalSign(s.config.Polynomial, equality, s.assignment)
	assignmentMust(t, err)
	if sign != 0 {
		t.Fatal("restored model", sign)
	}
}
