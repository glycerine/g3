package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func fmBlockVariable(t *testing.T, s *Solver, variable uint32) {
	t.Helper()
	p := solverPolynomial(t, s, []int32{1}, [][]polynomial.MonomialPower{{{Var: variable, Degree: 1}}})
	lit, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	b, err := s.MkBoolVar()
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{lit, sat.MakeLiteral(b, false)}, Assumption{}))
}

func TestSimplifyFourierMotzkinEqualitySubstitutesMixedClause(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	} // y,x
	equality := fmEqualityPolynomial(t, s, 1, 1, -2)
	solverUnit(t, s, AtomEQ, equality)
	xy := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 1, Degree: 1}}, {{Var: 0, Degree: 1}}})
	solverUnit(t, s, AtomGT, xy)
	fmBlockVariable(t, s, 0)
	count, _, err := s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	if count != 1 {
		t.Fatal("count", count)
	}
	want := solverPolynomial(t, s, []int32{1, 2}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, nil})
	core, err := s.config.Polynomial.ArithmeticCore()
	assignmentMust(t, err)
	found := false
	for _, clause := range s.clauses.originals {
		if clause.Size() != 1 || clause.Literals()[0] == TrueLiteral {
			continue
		}
		raw, cause := s.clauses.literalAtom(clause.Literals()[0])
		assignmentMust(t, cause)
		atom, ok := raw.(*InequalityAtom)
		if !ok || atom.Size() != 1 {
			continue
		}
		p, cause := atom.Polynomial(0)
		assignmentMust(t, cause)
		equal, cause := core.Equal(p, want)
		assignmentMust(t, cause)
		found = found || equal
	}
	if !found {
		t.Fatal("substituted y+2 literal missing")
	}
}
