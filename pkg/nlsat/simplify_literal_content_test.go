package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func contentSimplificationFixture(t *testing.T, integer bool, kind AtomKind, coefficients ...int32) (*Solver, polynomial.Polynomial, sat.Literal, *Clause) {
	t.Helper()
	s := solverFixture(t)
	_, err := s.MkVar(integer)
	assignmentMust(t, err)
	powers := make([][]polynomial.MonomialPower, len(coefficients))
	if len(powers) > 0 {
		powers[0] = []polynomial.MonomialPower{{Var: 0, Degree: 1}}
	}
	p := solverPolynomial(t, s, coefficients, powers)
	literal, err := s.MkInequalityLiteral(kind, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{literal}, Assumption{}))
	return s, p, literal, s.clauses.originals[len(s.clauses.originals)-1]
}

func contentReplacementPolynomial(t *testing.T, s *Solver, clause *Clause) polynomial.Polynomial {
	t.Helper()
	if clause.Size() != 1 {
		t.Fatalf("replacement clause size %d", clause.Size())
	}
	atom, err := s.clauses.literalAtom(clause.Literals()[0])
	assignmentMust(t, err)
	ineq, ok := atom.(*InequalityAtom)
	if !ok {
		t.Fatalf("replacement is %T", atom)
	}
	p, err := ineq.Polynomial(0)
	assignmentMust(t, err)
	return p
}

func contentWantPolynomial(t *testing.T, s *Solver, got polynomial.Polynomial, coefficients ...int32) {
	t.Helper()
	powers := make([][]polynomial.MonomialPower, len(coefficients))
	if len(powers) > 0 {
		powers[0] = []polynomial.MonomialPower{{Var: 0, Degree: 1}}
	}
	want := solverPolynomial(t, s, coefficients, powers)
	core, err := s.config.Polynomial.ArithmeticCore()
	assignmentMust(t, err)
	equal, err := core.Equal(got, want)
	assignmentMust(t, err)
	if !equal {
		t.Fatal("unexpected normalized polynomial")
	}
}

func TestSimplifyLiteralContentEqualityDividesAllCoefficients(t *testing.T) {
	s, _, oldLiteral, oldClause := contentSimplificationFixture(t, true, AtomEQ, 6, 9)
	count, err := s.SimplifyLiteralContent()
	assignmentMust(t, err)
	if count != 1 || s.clauses.owned[oldClause] != nil {
		t.Fatal("rewrite", count, s.clauses.owned[oldClause])
	}
	clause := s.clauses.originals[len(s.clauses.originals)-1]
	if clause.Literals()[0].Variable() == oldLiteral.Variable() {
		t.Fatal("atom was not canonicalized")
	}
	atom, err := s.clauses.literalAtom(clause.Literals()[0])
	assignmentMust(t, err)
	if arithmeticAtomHeader(atom).RefCount() != 1 {
		t.Fatal("temporary literal ownership retained", arithmeticAtomHeader(atom).RefCount())
	}
	contentWantPolynomial(t, s, contentReplacementPolynomial(t, s, clause), 2, 3)
}
