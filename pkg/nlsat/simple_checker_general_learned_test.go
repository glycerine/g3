package nlsat

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestSimpleCheckPolynomialSignsLearnsImpossibleDisjunct(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 3; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	simplePositiveUnits(t, s, 2)
	p := solverPolynomial(t, s, []int32{1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}, {Var: 1, Degree: 1}}})
	bad, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	z := fmEqualityPolynomial(t, s, 2, 1, 0)
	good, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{z}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{bad, good}, Assumption{}))
	ok, learned, err := s.SimpleCheckPolynomialSigns()
	assignmentMust(t, err)
	found := false
	for _, l := range learned {
		found = found || l == bad.Negated()
	}
	if !ok || !found {
		t.Fatal(ok, learned)
	}
}
