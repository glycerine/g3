package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyDriverSplitsFactorsOnlyAfterShrink(t *testing.T) {
	s, p, q, clause := factorSimplificationFixture(t, AtomEQ, false, [2]bool{}, Assumption{})
	_ = p
	_ = q
	fmBlockVariable(t, s, 0)
	fmBlockVariable(t, s, 1)
	assignmentMust(t, s.AddClause(append([]sat.Literal(nil), clause.Literals()...), Assumption{}))
	summary, err := s.Simplify()
	assignmentMust(t, err)
	if summary.Subsumed == 0 || summary.FactorsSplit != 1 || summary.Rounds < 2 {
		t.Fatal("schedule", summary)
	}
}
