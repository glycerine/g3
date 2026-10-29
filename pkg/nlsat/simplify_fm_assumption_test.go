package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyFourierMotzkinResolventJoinsAssumptions(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 3; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	// Variables y,z,x use indices 0,1,2 so descending elimination visits x.
	lower := solverPolynomial(t, s, []int32{1, -1}, [][]polynomial.MonomialPower{{{Var: 2, Degree: 1}}, {{Var: 0, Degree: 1}}})
	upper := solverPolynomial(t, s, []int32{-1, 1}, [][]polynomial.MonomialPower{{{Var: 2, Degree: 1}}, {{Var: 1, Degree: 1}}})
	loLit, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{lower}, []bool{false})
	assignmentMust(t, err)
	hiLit, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{upper}, []bool{false})
	assignmentMust(t, err)
	first, second := &AssumptionAttachment{}, &AssumptionAttachment{}
	assignmentMust(t, s.AddClause([]sat.Literal{loLit}, AttachedAssumption(first)))
	assignmentMust(t, s.AddClause([]sat.Literal{hiLit}, AttachedAssumption(second)))
	plain, err := s.MkBoolVar()
	assignmentMust(t, err)
	for variable := uint32(0); variable < 2; variable++ {
		blocker := solverPolynomial(t, s, []int32{1}, [][]polynomial.MonomialPower{{{Var: variable, Degree: 1}}})
		blockerLit, cause := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{blocker}, []bool{false})
		assignmentMust(t, cause)
		assignmentMust(t, s.AddClause([]sat.Literal{blockerLit, sat.MakeLiteral(plain, false)}, Assumption{}))
	}
	count, supported, err := s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	if !supported || count != 1 {
		t.Fatal("elimination", count, supported)
	}
	resolvent := s.clauses.originals[len(s.clauses.originals)-1]
	var assumptions []Assumption
	assignmentMust(t, s.assumptions.linearize(resolvent.Assumptions(), &assumptions))
	found := map[*AssumptionAttachment]bool{}
	for _, value := range assumptions {
		found[value.Attachment()] = true
	}
	if len(assumptions) != 2 || !found[first] || !found[second] {
		t.Fatal("joined assumptions", assumptions)
	}
}
