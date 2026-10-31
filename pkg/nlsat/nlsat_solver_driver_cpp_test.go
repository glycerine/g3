package nlsat

// Source: src/test/nlsat.cpp tst7.
import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestNLSATSourceAssumptionsConflictAndModel(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 7; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	polys := []polynomial.Polynomial{
		solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 4, Degree: 1}}, {{Var: 5, Degree: 1}}}),
		solverPolynomial(t, s, []int32{1, -1}, [][]polynomial.MonomialPower{{{Var: 6, Degree: 1}}, {{Var: 4, Degree: 1}}}),
		solverPolynomial(t, s, []int32{-1, -1}, [][]polynomial.MonomialPower{{{Var: 4, Degree: 1}}, {{Var: 5, Degree: 1}}}),
	}
	lits := make([]sat.Literal, len(polys))
	for i, p := range polys {
		var err error
		lits[i], err = s.MkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
		assignmentMust(t, err)
		assignmentMust(t, s.RetainLiteral(lits[i]))
		defer func(lit sat.Literal) { assignmentMust(t, s.ReleaseLiteral(lit)) }(lits[i])
	}
	assumptions := append([]sat.Literal(nil), lits...)
	status, err := s.CheckAssumptions(&assumptions)
	assignmentMust(t, err)
	if status != lbool.False {
		t.Fatal("contradictory inequalities", status)
	}
	assumptions = append([]sat.Literal(nil), lits[:2]...)
	status, err = s.CheckAssumptions(&assumptions)
	assignmentMust(t, err)
	if status != lbool.True {
		t.Fatal("satisfiable prefix", status)
	}
	for _, lit := range lits[:2] {
		v, err := s.LiteralValue(lit)
		assignmentMust(t, err)
		if v != lbool.True {
			t.Fatal("model violates assumed literal", lit, v)
		}
	}
}
