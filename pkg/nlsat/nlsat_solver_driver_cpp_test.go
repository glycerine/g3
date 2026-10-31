package nlsat

// Source: src/test/nlsat.cpp tst7.
import (
	"bytes"
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

// Source: src/test/nlsat.cpp tst6 and tst8, currently using signed projection.
func TestNLSATSourceProjectionDriver(t *testing.T) {
	t.Run("four inequalities", func(t *testing.T) {
		s := solverFixture(t)
		for i := 0; i < 7; i++ {
			_, err := s.MkVar(false)
			assignmentMust(t, err)
		}
		p := []polynomial.Polynomial{
			solverPolynomial(t, s, []int32{1, 1, 2}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}, {Var: 4, Degree: 2}}, {{Var: 6, Degree: 1}}, nil}),
			solverPolynomial(t, s, []int32{1, -2, -1, 8}, [][]polynomial.MonomialPower{{{Var: 1, Degree: 1}, {Var: 5, Degree: 1}}, {{Var: 6, Degree: 1}}, {{Var: 4, Degree: 1}}, nil}),
			solverPolynomial(t, s, []int32{1, 1, 1}, [][]polynomial.MonomialPower{{{Var: 2, Degree: 1}, {Var: 4, Degree: 1}}, {{Var: 6, Degree: 1}}, nil}),
			solverPolynomial(t, s, []int32{1, -1, 5}, [][]polynomial.MonomialPower{{{Var: 3, Degree: 1}, {Var: 4, Degree: 1}}, {{Var: 5, Degree: 1}}, {{Var: 6, Degree: 1}}}),
		}
		for i, v := range []int64{1, 1, 2, 2, 2, 1, 1} {
			evaluatorSetFraction(t, s.evaluator, uint32(i), v, 1)
		}
		lits := make([]sat.Literal, len(p))
		for i, poly := range p {
			var err error
			lits[i], err = s.MkInequality(AtomGT, []polynomial.Polynomial{poly}, []bool{false}, false)
			assignmentMust(t, err)
			assignmentMust(t, s.RetainLiteral(lits[i]))
			defer func(lit sat.Literal) { assignmentMust(t, s.ReleaseLiteral(lit)) }(lits[i])
		}
		for _, tc := range []struct {
			variable     uint32
			start, count int
		}{{4, 0, 2}, {5, 0, 3}, {6, 0, 3}, {6, 0, 2}, {6, 0, 4}, {6, 1, 3}} {
			checkSourceProjection(t, s, tc.variable, lits[tc.start:tc.start+tc.count])
		}
	})
	t.Run("quadratic equality", func(t *testing.T) {
		s := solverFixture(t)
		for i := 0; i < 7; i++ {
			_, err := s.MkVar(false)
			assignmentMust(t, err)
		}
		p := solverPolynomial(t, s, []int32{1, -1, -1}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}, {Var: 6, Degree: 2}}, {{Var: 1, Degree: 1}, {Var: 6, Degree: 1}}, {{Var: 2, Degree: 1}}})
		for i, v := range []int64{2, 1, 6, 0, 0, 0, 2} {
			evaluatorSetFraction(t, s.evaluator, uint32(i), v, 1)
		}
		lit, err := s.MkInequality(AtomEQ, []polynomial.Polynomial{p}, []bool{false}, false)
		assignmentMust(t, err)
		assignmentMust(t, s.RetainLiteral(lit))
		defer func() { assignmentMust(t, s.ReleaseLiteral(lit)) }()
		checkSourceProjection(t, s, 6, []sat.Literal{lit})
	})
}

func checkSourceProjection(t *testing.T, s *Solver, variable uint32, input []sat.Literal) {
	t.Helper()
	result, err := NewLiteralVector(s)
	assignmentMust(t, err)
	defer func() { assignmentMust(t, result.Close()) }()
	if err := s.Project(variable, input, result); err != nil {
		t.Fatalf("project: %v", err)
	}
	var projected []sat.Literal
	if err := result.Literals(&projected); err != nil {
		t.Fatalf("result literals: %v", err)
	}
	for _, lit := range projected {
		value, err := s.LiteralValue(lit)
		assignmentMust(t, err)
		if value != lbool.True {
			var buf bytes.Buffer
			s.DisplayLiteral(&buf, lit)
			t.Fatal("projected literal false in source model", lit, value, buf.String())
		}
		var variables []uint32
		if err := s.LiteralVariables(lit, &variables); err != nil {
			t.Fatalf("literal variables: %v", err)
		}
		for _, v := range variables {
			if v == variable {
				t.Fatal("projection still contains eliminated variable", variable, variables)
			}
		}
	}
}
