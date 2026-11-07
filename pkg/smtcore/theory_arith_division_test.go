package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTArithmeticDivisionScopedProofs(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, tc := range []struct {
			op            int32
			denominator   string
			comparison    int32
			bound         string
			boundDividend bool
		}{
			{smtArithOpIDiv, "3", smtArithOpLE, "0", true},
			{smtArithOpMod, "3", smtArithOpLT, "0", false},
			{smtArithOpMod, "-3", smtArithOpGE, "3", false},
			{smtArithOpRem, "-3", smtArithOpGT, "0", false},
		} {
			t.Run(fmt.Sprint(proofs, "/", tc), func(t *testing.T) {
				m, cfg := smtModelGeneratorFixture(t)
				if proofs {
					astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				}
				solver, err := NewSMTSolver(m, nil, "QF_LIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, solver.Close()) })
				// Preserve the division applications to exercise the backend
				// axioms independently of the arithmetic preprocessor.
				solver.kernel.preprocess = false
				family, err := m.ArithmeticFamilyID()
				astTestMust(t, err)
				sort, err := m.MkFamilySort(family, 1, nil)
				astTestMust(t, err)
				x, err := m.MkConst(astTestSymbol(t, m, "division-x"), sort)
				astTestMust(t, err)
				d, err := m.ArithmeticIntegerNumeralText(tc.denominator)
				astTestMust(t, err)
				term, err := m.MkFamilyApp(family, tc.op, nil, []ast.ASTHandle{x, d})
				astTestMust(t, err)
				bound, err := m.ArithmeticIntegerNumeralText(tc.bound)
				astTestMust(t, err)
				formula, err := m.MkFamilyApp(family, tc.comparison, nil, []ast.ASTHandle{term, bound})
				astTestMust(t, err)
				formulas := []ast.ASTHandle{formula}
				if tc.boundDividend {
					four, err := m.ArithmeticIntegerNumeralText("4")
					astTestMust(t, err)
					lower, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, four})
					astTestMust(t, err)
					formulas = append(formulas, lower)
				}
				for _, h := range formulas {
					ref, err := m.NewRef(h)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
				}
				for repeat := 0; repeat < 2; repeat++ {
					astTestMust(t, solver.Push())
					for _, h := range formulas {
						astTestMust(t, solver.Assert(h))
					}
					status, err := solver.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.False {
						t.Fatal("division contradiction", status)
					}
					if proofs {
						proof, err := solver.Proof()
						astTestMust(t, err)
						fact, err := m.GetProofFact(proof)
						astTestMust(t, err)
						if !m.IsFalse(fact) {
							t.Fatal("division contradiction proof")
						}
					}
					astTestMust(t, solver.Pop(1))
					theory := solver.kernel.internalizer.theories[family].(*SMTLinearArithmeticTheory)
					if len(theory.divisionTerms) != 0 {
						t.Fatal("division axiom trail survived scope pop")
					}
					status, err = solver.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("division scope retained conflict", status)
					}
				}
			})
		}
	}
}

func TestSMTArithmeticSymbolicZeroDivisorGuard(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	solver.kernel.preprocess = false
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	sort, err := m.GetSort(zero)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "guarded-dividend"), sort)
	astTestMust(t, err)
	d, err := m.MkConst(astTestSymbol(t, m, "guarded-divisor"), sort)
	astTestMust(t, err)
	division, err := m.MkFamilyApp(family, smtArithOpIDiv, nil, []ast.ASTHandle{x, d})
	astTestMust(t, err)
	five, err := m.ArithmeticIntegerNumeralText("5")
	astTestMust(t, err)
	quotient, err := m.MkEq(division, five)
	astTestMust(t, err)
	ref, err := m.NewRef(quotient)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
	guard, err := m.MkEq(d, zero)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(guard))
	astTestMust(t, solver.Assert(quotient))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		reason, _ := solver.ReasonUnknown()
		t.Fatal("zero divisor must disable the division equation", status, reason)
	}
}

func TestSMTArithmeticUnderspecifiedParentMarksChildShared(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	smtDisableAutoConfig(t, solver)
	solver.kernel.preprocess = false
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	sort, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "shared-division-child"), sort)
	astTestMust(t, err)
	z, err := m.MkConst(astTestSymbol(t, m, "ordinary-arithmetic-child"), sort)
	astTestMust(t, err)
	division, err := m.MkFamilyApp(family, smtArithOpIDiv, nil, []ast.ASTHandle{x, one})
	astTestMust(t, err)
	ordinary, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{z, one})
	astTestMust(t, err)
	state := solver.kernel.internalizer
	astTestMust(t, state.Internalize(division, false))
	astTestMust(t, state.Internalize(ordinary, false))
	theory, ok := state.theories[family].(*SMTLinearArithmeticTheory)
	if !ok || len(theory.divisionTerms) == 0 {
		t.Fatal("division fixture did not reach the underspecified-operator owner")
	}
	shared, err := theory.sharedArithmeticNode(state.context.enodes.get(x))
	astTestMust(t, err)
	if !shared {
		t.Fatal("division child was not marked shared")
	}
	shared, err = theory.sharedArithmeticNode(state.context.enodes.get(z))
	astTestMust(t, err)
	if shared {
		t.Fatal("ordinary single-theory arithmetic child was marked shared")
	}
}
