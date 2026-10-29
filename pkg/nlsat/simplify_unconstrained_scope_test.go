package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyUnconstrainedEqualitiesRetainsRejectedForms(t *testing.T) {
	t.Run("multiple clause occurrences", func(t *testing.T) {
		s := solverFixture(t)
		_, err := s.MkVar(false)
		assignmentMust(t, err)
		first := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
		second := solverPolynomial(t, s, []int32{1, -1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
		solverUnit(t, s, AtomEQ, first)
		solverUnit(t, s, AtomGT, second)
		count, supported, err := s.SimplifyUnconstrainedEqualities()
		assignmentMust(t, err)
		if !supported || count != 0 || len(s.patches) != 0 {
			t.Fatal(count, supported, len(s.patches))
		}
	})
	t.Run("symbolic coefficient", func(t *testing.T) {
		s := solverFixture(t)
		for i := 0; i < 2; i++ {
			_, err := s.MkVar(false)
			assignmentMust(t, err)
		}
		xyMinusOne := solverPolynomial(t, s, []int32{1, -1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}, {Var: 1, Degree: 1}}, nil})
		solverUnit(t, s, AtomEQ, xyMinusOne)
		count, supported, err := s.SimplifyUnconstrainedEqualities()
		assignmentMust(t, err)
		if !supported || count != 0 || len(s.patches) != 0 {
			t.Fatal(count, supported, len(s.patches))
		}
	})
	t.Run("negative equality and even factor", func(t *testing.T) {
		s := solverFixture(t)
		_, err := s.MkVar(false)
		assignmentMust(t, err)
		p := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
		negative, err := s.MkInequality(AtomEQ, []polynomial.Polynomial{p}, []bool{false}, true)
		assignmentMust(t, err)
		even, err := s.MkInequality(AtomEQ, []polynomial.Polynomial{p}, []bool{true}, false)
		assignmentMust(t, err)
		assignmentMust(t, s.AddClause([]sat.Literal{negative, even}, Assumption{}))
		count, supported, err := s.SimplifyUnconstrainedEqualities()
		assignmentMust(t, err)
		if !supported || count != 0 || len(s.patches) != 0 {
			t.Fatal(count, supported, len(s.patches))
		}
	})
}
