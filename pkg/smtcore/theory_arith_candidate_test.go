package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTArithmeticCandidateFloorsFractionalIntegers(t *testing.T) {
	for _, test := range []struct {
		name, sum, want string
		status          lbool.LBool
	}{
		{"positive-half", "1", "0", lbool.Undef},
		{"negative-half", "-1", "-1", lbool.Undef},
		{"integral", "2", "1", lbool.True},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			params, err := config.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, config.Context.Del(&params)) })
			for _, option := range []struct {
				name  string
				value bool
			}{{"preprocess", false}, {"arith.ignore_int", true}} {
				astTestMust(t, config.Context.SetBoolName(&params, &option.name, option.value))
			}
			s, err := NewSMTSolver(m, &params, "QF_LIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()) })
			astTestMust(t, s.SetProduceModels(true))
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			integer, err := m.GetSort(zero)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "fractional-x"), integer)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "fractional-y"), integer)
			astTestMust(t, err)
			// Preprocessing is off: give theory_arith the rewriter's normal
			// form of (- x y), (+ x (* -1 y)) (internalize_term_core rejects
			// subtraction).
			minusOne, err := m.ArithmeticIntegerNumeralText("-1")
			astTestMust(t, err)
			negated, err := m.MkArithmeticMul(minusOne, y)
			astTestMust(t, err)
			difference, err := m.MkArithmeticAdd(x, negated)
			astTestMust(t, err)
			same, err := m.MkEq(difference, zero)
			astTestMust(t, err)
			astTestMust(t, s.Assert(same))
			sum, err := m.MkArithmeticAdd(x, y)
			astTestMust(t, err)
			total, err := m.ArithmeticIntegerNumeralText(test.sum)
			astTestMust(t, err)
			equation, err := m.MkEq(sum, total)
			astTestMust(t, err)
			astTestMust(t, s.Assert(equation))
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != test.status {
				t.Fatal("integer candidate status", status, test.status)
			}
			if status == lbool.Undef {
				if s.kernel.lastFailure != SMTFailureTheory {
					t.Fatal("fractional integer did not give up", s.kernel.lastFailure)
				}
				available, err := s.PrepareCandidateModel()
				astTestMust(t, err)
				if !available {
					t.Fatal("fractional candidate unavailable")
				}
			}
			generated, err := s.AcquireModel()
			astTestMust(t, err)
			if generated == nil {
				t.Fatal("missing integer model")
			}
			defer func() { astTestMust(t, generated.DecRef()) }()
			for _, expression := range []ast.ASTHandle{x, y} {
				declaration, err := m.GetDecl(expression)
				astTestMust(t, err)
				value, err := generated.ConstantInterpretation(declaration)
				astTestMust(t, err)
				text, integer, numeral, err := m.ArithmeticNumeralText(value)
				astTestMust(t, err)
				if !numeral || !integer || text != test.want {
					t.Fatal("floored integer model value", text, integer, numeral, test.want)
				}
			}
		})
	}
}
