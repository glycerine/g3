package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSimplifyFourierMotzkinLeavesUnsupportedVariableUntouched(t *testing.T) {
	for _, test := range []struct {
		name        string
		integer     bool
		coefficient int32
		mixed       bool
	}{
		{"integer-nonunit", true, 2, false}, {"mixed-clause", false, 1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := solverFixture(t)
			_, err := s.MkVar(test.integer)
			assignmentMust(t, err)
			p := solverPolynomial(t, s, []int32{test.coefficient, -1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
			lit, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{p}, []bool{false})
			assignmentMust(t, err)
			literals := []sat.Literal{lit}
			if test.mixed {
				b, cause := s.MkBoolVar()
				assignmentMust(t, cause)
				literals = append(literals, sat.MakeLiteral(b, false))
			}
			assignmentMust(t, s.AddClause(literals, Assumption{}))
			before := s.clauses.originals[len(s.clauses.originals)-1]
			count, supported, err := s.SimplifyFourierMotzkin()
			assignmentMust(t, err)
			if !supported || count != 0 || s.clauses.owned[before] == nil || len(s.fmBounds) != 0 {
				t.Fatal(count, supported, len(s.fmBounds))
			}
		})
	}
}
