package smtcore

// Source: src/smt/smt_setup.cpp setup_mi_arith callers.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTMixedSetupSelectsInfiniteArithmetic(t *testing.T) {
	for _, logic := range []string{"QF_RDL", "LRA", "AUFLIRA"} {
		t.Run(logic, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			params, err := cfg.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
			name := "arith.solver"
			astTestMust(t, cfg.Context.SetUIntName(&params, &name, 5))
			solver, err := NewSMTSolver(m, &params, logic, SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })

			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			real, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "mixed-infinite-x"), real)
			astTestMust(t, err)
			zero, err := m.ArithmeticRealNumeralText("0")
			astTestMust(t, err)
			bound, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, zero})
			astTestMust(t, err)
			astTestMust(t, solver.Assert(bound))
			// Ordinary CheckSat selects CFG_LOGIC and therefore the source
			// logic-only setup_mi_arith path rather than the static overload.
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("mixed setup changed satisfiability", status)
			}
			theory, ok := solver.kernel.internalizer.theories[family].(*SMTTheoryArith)
			if !ok || !theory.infExt {
				t.Fatalf("setup_mi_arith mode 5 selected %T", solver.kernel.internalizer.theories[family])
			}
		})
	}
}

func TestSMTRDLStaticSetupKeepsDirectBackendSelection(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	params, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
	name := "arith.solver"
	astTestMust(t, cfg.Context.SetUIntName(&params, &name, 5))
	solver, err := NewSMTSolver(m, &params, "QF_RDL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })

	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "static-rdl-x"), real)
	astTestMust(t, err)
	zero, err := m.ArithmeticRealNumeralText("0")
	astTestMust(t, err)
	bound, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, zero})
	astTestMust(t, err)
	astTestMust(t, solver.Assert(bound))
	status, err := smtSetupProfileCheck(solver)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	theory, ok := solver.kernel.internalizer.theories[family].(*SMTTheoryArith)
	if !ok || theory.infExt {
		t.Fatalf("static QF_RDL incorrectly used setup_mi_arith: %T", solver.kernel.internalizer.theories[family])
	}
}
