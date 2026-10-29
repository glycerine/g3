package nlsat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func contentBigPolynomial(t *testing.T, s *Solver, coefficients ...string) polynomial.Polynomial {
	t.Helper()
	core, err := s.config.Polynomial.ArithmeticCore()
	assignmentMust(t, err)
	mm, err := core.MonomialManager()
	assignmentMust(t, err)
	z, err := core.IntegerManager()
	assignmentMust(t, err)
	values := make([]unumeral.Int, len(coefficients))
	terms := make([]polynomial.Monomial, len(coefficients))
	for i, coefficient := range coefficients {
		assignmentMust(t, z.SetString(&values[i], coefficient))
		if i == 0 {
			terms[i], err = mm.MkMonomial([]polynomial.MonomialPower{{Var: 0, Degree: 1}})
		} else {
			terms[i], err = mm.Unit()
		}
		assignmentMust(t, err)
	}
	p, err := core.MkTerms(values, terms)
	for i := range values {
		err = errors.Join(err, z.Del(&values[i]))
	}
	assignmentMust(t, err)
	assignmentMust(t, core.IncRef(p))
	t.Cleanup(func() { assignmentMust(t, core.DecRef(p)) })
	return p
}

func TestSimplifyLiteralContentArbitraryPrecisionGCD(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(true)
	assignmentMust(t, err)
	p := contentBigPolynomial(t, s, "36893488147419103234", "55340232221128654851")
	literal, err := s.MkInequalityLiteral(AtomEQ, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{literal}, Assumption{}))
	count, err := s.SimplifyLiteralContent()
	assignmentMust(t, err)
	if count != 1 {
		t.Fatal("big content rewrite", count)
	}
	clause := s.clauses.originals[len(s.clauses.originals)-1]
	contentWantPolynomial(t, s, contentReplacementPolynomial(t, s, clause), 2, 3)
}

func TestSimplifyLiteralContentArbitraryPrecisionSlowPathHonorsUnit(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(true)
	assignmentMust(t, err)
	p := contentBigPolynomial(t, s, "36893488147419103234", "1")
	literal, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{literal}, Assumption{}))
	old := s.clauses.originals[len(s.clauses.originals)-1]
	count, err := s.SimplifyLiteralContent()
	assignmentMust(t, err)
	if count != 0 || s.clauses.owned[old] == nil || old.Literals()[0] != literal {
		t.Fatal("slow unit path changed", count)
	}
}
