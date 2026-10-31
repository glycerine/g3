package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestReorderRootAtomPreservesValueAndInterning(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, e := s.MkVar(false)
		assignmentMust(t, e)
	}
	p := solverPolynomial(t, s, []int32{1, -2}, [][]polynomial.MonomialPower{{{Var: 1, Degree: 1}}, nil})
	for i, v := range []int64{0, 3} {
		evaluatorSetFraction(t, s.evaluator, uint32(i), v, 1)
	}
	lit, e := s.MkRoot(AtomRootGT, 1, 1, p, false)
	assignmentMust(t, e)
	assignmentMust(t, s.RetainLiteral(lit))
	defer func() { assignmentMust(t, s.ReleaseLiteral(lit)) }()
	check := func(x uint32) {
		t.Helper()
		a, found, e := s.AtomForBooleanVariable(lit.Variable())
		assignmentMust(t, e)
		root, ok := a.(*RootAtom)
		if !found || !ok || root.Variable() != x {
			t.Fatal("root position", found, a, x)
		}
		value, e := s.LiteralValue(lit)
		assignmentMust(t, e)
		if value != lbool.True {
			t.Fatal("root model value", value)
		}
		again, e := s.MkRoot(AtomRootGT, x, 1, p, false)
		assignmentMust(t, e)
		if again != lit {
			t.Fatal("root interning changed", again, lit)
		}
	}
	check(1)
	assignmentMust(t, s.ReorderVariables([]uint32{1, 0}))
	check(0)
	assignmentMust(t, s.RestoreVariableOrder())
	check(1)
}

func TestReorderDeletesLearnedClausesWithIllFormedRoots(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, e := s.MkVar(false)
		assignmentMust(t, e)
	}
	p := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, {{Var: 1, Degree: 1}}})
	lit, e := s.MkRoot(AtomRootGT, 1, 1, p, false)
	assignmentMust(t, e)
	assignmentMust(t, s.RetainLiteral(lit))
	defer func() { assignmentMust(t, s.ReleaseLiteral(lit)) }()
	_, e = s.clauses.mkClause([]sat.Literal{lit}, true, nil)
	assignmentMust(t, e)
	if len(s.clauses.learned) != 1 {
		t.Fatal("learned fixture")
	}
	assignmentMust(t, s.ReorderVariables([]uint32{1, 0}))
	if len(s.clauses.learned) != 0 {
		t.Fatal("ill-formed root clause survived reorder")
	}
	assignmentMust(t, s.RestoreVariableOrder())
}
