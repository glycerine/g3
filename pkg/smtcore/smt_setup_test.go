package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Algorithm-focused tests set theory controls directly; disable automatic
// setup so the driver's first push cannot replace those chosen controls.
func smtDisableAutoConfig(t *testing.T, solver *SMTSolver) {
	t.Helper()
	context := solver.kernel.paramContext
	params, err := context.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Del(&params)) }()
	name := "auto_config"
	astTestMust(t, context.SetBoolName(&params, &name, false))
	astTestMust(t, solver.UpdateParams(&params))
}

func TestSMTQFUFLIASetupOwnsInternalArithmeticThresholdAcrossUpdates(t *testing.T) {
	for _, tc := range []struct {
		name, logic string
		auto        bool
		threshold   uint32
	}{
		{"uflia-profile", "QF_UFLIA", true, 1000},
		{"basic-configuration", "QF_UFLIA", false, ^uint32(0)},
		{"other-integer-logic", "QF_LIA", true, ^uint32(0)},
		{"real-logic", "QF_UFLRA", true, ^uint32(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			params, err := cfg.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
			name := "auto_config"
			astTestMust(t, cfg.Context.SetBoolName(&params, &name, tc.auto))
			s, err := NewSMTSolver(m, &params, tc.logic, SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()) })
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			facts := func() *smtArithSetupFacts {
				f, _ := smtArithFactsOf(s.kernel.internalizer.theories[family])
				return f
			}
			checkThreshold := func() {
				theory := facts()
				owner, err := s.kernel.smtParams.ArithmeticState()
				astTestMust(t, err)
				if owner.PropagationThreshold != tc.threshold || s.kernel.arithState.PropagationThreshold != tc.threshold || theory.propagationThreshold != tc.threshold {
					t.Fatal("setup threshold missing from owner, cache, or arithmetic theory", owner.PropagationThreshold, s.kernel.arithState.PropagationThreshold, theory.propagationThreshold)
				}
			}
			checkThreshold()
			if tc.threshold == 1000 {
				// setup writes the parameter; the context level (2 until the
				// first setup_context) takes it at the first check or push.
				if s.kernel.smtParams.state.RelevancyLvl != 0 || s.kernel.smtState.RelevancyLvl != 0 || s.kernel.relevancyConfigured || s.kernel.internalizer.relevancyLevel != 2 {
					t.Fatal("QF_UFLIA relevancy setup was not applied")
				}
				if s.kernel.smtParams.arith.state.Reflect || s.kernel.arithState.Reflect || s.kernel.smtParams.preprocess.state.NNFCNF || s.kernel.preprocessState.NNFCNF {
					t.Fatal("QF_UFLIA reflection or preprocessing setup was not applied")
				}
			}
			name = "arith.propagate_eqs"
			for _, enabled := range []bool{false, true} {
				astTestMust(t, cfg.Context.SetBoolName(&params, &name, enabled))
				astTestMust(t, s.UpdateParams(&params))
				checkThreshold()
				if facts().propagateEqualities != enabled {
					t.Fatal("parameter update failed to reach the arithmetic theory")
				}
			}
		})
	}
}

func TestSMTQFUFBVSetupUsesNativeArrayAndBitVectorRoute(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	arrayFamily, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	bvFamily, err := m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_UFBV", SMTSolverConfig{
		Params:         modelConfig.Context,
		ModelEvaluator: modelConfig,
	})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	if _, ok := solver.kernel.internalizer.theories[arrayFamily].(*SMTArrayTheory); !ok {
		t.Fatal("QF_UFBV did not install the native shared array theory")
	}
	if _, ok := solver.kernel.internalizer.theories[bvFamily].(*SMTBitVectorTheory); !ok {
		t.Fatal("QF_UFBV did not install the bit-vector theory")
	}
}

func TestSMTQFAUFLIASetupUsesIntegerArithmeticAndArrays(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	arithmeticFamily, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	arrayFamily, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	integerSort, err := m.MkFamilySort(arithmeticFamily, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "qf-auflia-x"), integerSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "qf-auflia-y"), integerSort)
	astTestMust(t, err)
	less, err := m.MkFamilyApp(arithmeticFamily, smtArithOpLT, nil, []ast.ASTHandle{x, y})
	astTestMust(t, err)
	greaterEqual, err := m.MkFamilyApp(arithmeticFamily, smtArithOpGE, nil, []ast.ASTHandle{x, y})
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_AUFLIA", SMTSolverConfig{
		Params:         modelConfig.Context,
		ModelEvaluator: modelConfig,
	})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	arithmetic, ok := smtArithFactsOf(solver.kernel.internalizer.theories[arithmeticFamily])
	if !ok || !arithmetic.integer {
		t.Fatal("QF_AUFLIA did not install concrete integer linear arithmetic")
	}
	if _, ok = solver.kernel.internalizer.theories[arrayFamily].(*SMTArrayTheory); !ok {
		t.Fatal("QF_AUFLIA did not install the array theory")
	}
	astTestMust(t, solver.Assert(less))
	astTestMust(t, solver.Assert(greaterEqual))
	status, err := smtSetupProfileCheck(solver)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("QF_AUFLIA linear contradiction", status)
	}
}

func TestSMTLinearRealLogicSetupUsesConcreteArithmetic(t *testing.T) {
	for _, test := range []struct {
		logic      string
		wantArrays bool
	}{
		{logic: "LRA"},
		{logic: "UFLRA", wantArrays: true},
	} {
		t.Run(test.logic, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			arithmeticFamily, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			arrayFamily, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			realSort, err := m.MkFamilySort(arithmeticFamily, 0, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "linear-real-setup-x"), realSort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "linear-real-setup-y"), realSort)
			astTestMust(t, err)
			less, err := m.MkFamilyApp(arithmeticFamily, smtArithOpLT, nil, []ast.ASTHandle{x, y})
			astTestMust(t, err)
			greaterEqual, err := m.MkFamilyApp(arithmeticFamily, smtArithOpGE, nil, []ast.ASTHandle{x, y})
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, test.logic, SMTSolverConfig{
				Params:         modelConfig.Context,
				ModelEvaluator: modelConfig,
			})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			arithmetic, ok := smtArithFactsOf(solver.kernel.internalizer.theories[arithmeticFamily])
			if !ok || arithmetic.integer || !arithmetic.mixed {
				t.Fatal(test.logic, "did not install concrete mixed linear arithmetic")
			}
			_, hasArrays := solver.kernel.internalizer.theories[arrayFamily].(*SMTArrayTheory)
			if hasArrays != test.wantArrays {
				t.Fatal(test.logic, "array route", hasArrays, "want", test.wantArrays)
			}
			astTestMust(t, solver.Assert(less))
			astTestMust(t, solver.Assert(greaterEqual))
			status, err := smtSetupProfileCheck(solver)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal(test.logic, "linear contradiction", status)
			}
		})
	}
}

func TestSMTQuantifiedIntegerLogicSetupUsesConcreteLinearSubset(t *testing.T) {
	for _, logic := range []string{"AUFLIA", "AUFLIA+", "AUFLIA-", "UFNIA"} {
		t.Run(logic, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			arithmeticFamily, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			arrayFamily, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			integerSort, err := m.MkFamilySort(arithmeticFamily, 1, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "quantified-integer-setup-x"), integerSort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "quantified-integer-setup-y"), integerSort)
			astTestMust(t, err)
			less, err := m.MkFamilyApp(arithmeticFamily, smtArithOpLT, nil, []ast.ASTHandle{x, y})
			astTestMust(t, err)
			greaterEqual, err := m.MkFamilyApp(arithmeticFamily, smtArithOpGE, nil, []ast.ASTHandle{x, y})
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, logic, SMTSolverConfig{
				Params:         modelConfig.Context,
				ModelEvaluator: modelConfig,
			})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			arithmetic, ok := smtArithFactsOf(solver.kernel.internalizer.theories[arithmeticFamily])
			if !ok || !arithmetic.integer {
				t.Fatal(logic, "did not install the concrete integer linear subset")
			}
			if _, ok = solver.kernel.internalizer.theories[arrayFamily].(*SMTArrayTheory); !ok {
				t.Fatal(logic, "did not install the native shared array theory")
			}
			astTestMust(t, solver.Assert(less))
			astTestMust(t, solver.Assert(greaterEqual))
			status, err := smtSetupProfileCheck(solver)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal(logic, "linear contradiction", status)
			}
		})
	}
}

func TestSMTUnknownLogicSetupInstallsMixedArithmetic(t *testing.T) {
	for _, test := range []struct {
		logic   string
		integer bool
	}{
		{logic: "QF_NIA", integer: true},
		{logic: "QF_NRA"},
		{logic: "QF_UFNRA"},
	} {
		t.Run(test.logic, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			arithmeticFamily, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			arrayFamily, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			sortKind := int32(0)
			if test.integer {
				sortKind = 1
			}
			arithmeticSort, err := m.MkFamilySort(arithmeticFamily, sortKind, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "unknown-setup-x-"+test.logic), arithmeticSort)
			astTestMust(t, err)
			zero, err := m.ArithmeticNumeralRat(big.NewRat(0, 1), test.integer)
			astTestMust(t, err)
			product, err := m.MkFamilyApp(arithmeticFamily, smtArithOpMul, nil, []ast.ASTHandle{x, x})
			astTestMust(t, err)
			positive, err := m.MkFamilyApp(arithmeticFamily, smtArithOpGT, nil, []ast.ASTHandle{product, zero})
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, test.logic, SMTSolverConfig{
				Params:         modelConfig.Context,
				ModelEvaluator: modelConfig,
			})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			arithmetic, ok := smtArithFactsOf(solver.kernel.internalizer.theories[arithmeticFamily])
			if !ok || !arithmetic.mixed {
				t.Fatal(test.logic, "did not install mixed arithmetic")
			}
			if _, ok := solver.kernel.internalizer.theories[arrayFamily].(*SMTArrayTheory); !ok {
				t.Fatal(test.logic, "did not install setup_unknown array theory")
			}
			astTestMust(t, solver.Assert(positive))
			status, err := smtSetupProfileCheck(solver)
			astTestMust(t, err)
			// z3 4.7.1's smt kernel (no NLSAT inside it): x*x > 0 is sat over
			// the integers and unknown over the reals.
			want := lbool.Undef
			if test.integer {
				want = lbool.True
			}
			if status != want {
				t.Fatal(test.logic, "nonlinear arithmetic setup", status, want)
			}
		})
	}
}

func TestSMTAllSetupIncludesArrays(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	arrayFamily, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{
		Params:         modelConfig.Context,
		ModelEvaluator: modelConfig,
	})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	if _, ok := solver.kernel.internalizer.theories[arrayFamily].(*SMTArrayTheory); !ok {
		t.Fatal("ALL did not install setup_unknown array theory")
	}
}
