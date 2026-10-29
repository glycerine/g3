package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyFourierMotzkinEqualityJoinsClauseAssumption(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	eq := fmEqualityPolynomial(t, s, 1, 1, -2)
	eqLit, err := s.MkInequalityLiteral(AtomEQ, []polynomial.Polynomial{eq}, []bool{false})
	assignmentMust(t, err)
	first, second := &AssumptionAttachment{}, &AssumptionAttachment{}
	assignmentMust(t, s.AddClause([]sat.Literal{eqLit}, AttachedAssumption(first)))
	xy := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 1, Degree: 1}}, {{Var: 0, Degree: 1}}})
	xyLit, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{xy}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{xyLit}, AttachedAssumption(second)))
	fmBlockVariable(t, s, 0)
	_, _, err = s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	var joined *Clause
	for _, clause := range s.clauses.originals {
		if clause.Size() == 1 && clause.Literals()[0] != TrueLiteral {
			joined = clause
		}
	}
	if joined == nil {
		t.Fatal("replacement missing")
	}
	var values []Assumption
	assignmentMust(t, s.assumptions.linearize(joined.Assumptions(), &values))
	found := map[*AssumptionAttachment]bool{}
	for _, value := range values {
		found[value.Attachment()] = true
	}
	if !found[first] || !found[second] {
		t.Fatal("assumptions", values)
	}
}
