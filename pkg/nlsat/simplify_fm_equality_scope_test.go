package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyFourierMotzkinIntegerNonunitEqualityUntouched(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(true)
	assignmentMust(t, err)
	p := fmEqualityPolynomial(t, s, 0, 2, -4)
	lit, err := s.MkInequalityLiteral(AtomEQ, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{lit}, Assumption{}))
	old := s.clauses.originals[len(s.clauses.originals)-1]
	count, supported, err := s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	if !supported || count != 0 || s.clauses.owned[old] == nil || len(s.fmBounds) != 0 {
		t.Fatal(count, supported, len(s.fmBounds))
	}
}
