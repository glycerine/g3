package smtcore

import (
	"fmt"
	"strings"
	"testing"
)

func TestSMTGenericSequenceBackendUsesUpdatedOptionsAndFeatures(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		for _, choice := range []string{"seq", "z3str3", "auto"} {
			for _, nonString := range []bool{false, true} {
				for _, static := range []bool{false, true} {
					t.Run(fmt.Sprint(automatic, "/", choice, "/", nonString, "/", static), func(t *testing.T) {
						m, cfg := smtModelGeneratorFixture(t)
						params, err := cfg.Context.NewValue()
						astTestMust(t, err)
						t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
						solver, err := NewSMTSolver(m, &params, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
						astTestMust(t, err)
						t.Cleanup(func() { astTestMust(t, solver.Close()) })
						key := "auto_config"
						astTestMust(t, cfg.Context.SetBoolName(&params, &key, automatic))
						key = "string_solver"
						astTestMust(t, cfg.Context.SetSymbolName(&params, &key, astTestSymbol(t, m, choice)))
						astTestMust(t, solver.UpdateParams(&params))
						literal := astTestString(t, m, "a")
						if nonString {
							one, err := m.ArithmeticIntegerNumeralText("1")
							astTestMust(t, err)
							literal, err = m.MkSequenceUnit(one)
							astTestMust(t, err)
						}
						sort, err := m.GetSort(literal)
						astTestMust(t, err)
						x, err := m.MkConst(astTestSymbol(t, m, "setup-sequence-feature"), sort)
						astTestMust(t, err)
						equality, err := m.MkEq(x, literal)
						astTestMust(t, err)
						astTestMust(t, solver.Assert(equality))
						astTestMust(t, solver.kernel.setupAssertedBounds(static))
						family, err := m.SequenceFamilyID()
						astTestMust(t, err)
						plugin := solver.kernel.internalizer.theories[family]
						str, legacy := plugin.(*SMTStringTheory)
						if legacy != (choice == "z3str3" || choice == "auto" && !nonString) {
							t.Fatal("generic selection ignored updated parameters or asserted sort features", plugin)
						}
						if legacy && (str.parameterOwner != &solver.kernel.smtParams.str || str.regexExpressionSolver.kernel.smtParams != solver.kernel.smtParams) {
							t.Fatal("selected String plugin lost its borrowed parameter owners")
						}
						// The native setup boundary is permanent, including later
						// updates and scopes; do not replace an active theory.
						astTestMust(t, cfg.Context.SetSymbolName(&params, &key, astTestSymbol(t, m, "invalid-after-setup")))
						astTestMust(t, solver.UpdateParams(&params))
						astTestMust(t, solver.kernel.setupAssertedBounds(!static))
						if plugin != solver.kernel.internalizer.theories[family] {
							t.Fatal("completed setup replaced its sequence plugin")
						}
					})
				}
			}
		}
	}
}

func TestSMTSequenceBackendInvalidSelectorAndQFSLateModeChange(t *testing.T) {
	for _, initialAuto := range []bool{false, true} {
		m, cfg := smtModelGeneratorFixture(t)
		params, err := cfg.Context.NewValue()
		astTestMust(t, err)
		key := "auto_config"
		astTestMust(t, cfg.Context.SetBoolName(&params, &key, initialAuto))
		solver, err := NewSMTSolver(m, &params, "QF_S", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
		astTestMust(t, err)
		astTestMust(t, cfg.Context.SetBoolName(&params, &key, !initialAuto))
		key = "string_solver"
		astTestMust(t, cfg.Context.SetSymbolName(&params, &key, astTestSymbol(t, m, "invalid-selector")))
		astTestMust(t, solver.UpdateParams(&params))
		err = solver.Push()
		if initialAuto {
			if err == nil || !strings.Contains(err.Error(), "invalid parameter for smt.string_solver") || solver.kernel.preprocessingSetupDone {
				t.Fatal("CFG_BASIC accepted invalid generic selector", err)
			}
			astTestMust(t, cfg.Context.SetSymbolName(&params, &key, astTestSymbol(t, m, "seq")))
			astTestMust(t, solver.UpdateParams(&params))
			astTestMust(t, solver.Push())
		} else {
			astTestMust(t, err) // QF_S ignores the generic selector.
		}
		family, err := m.SequenceFamilyID()
		astTestMust(t, err)
		plugin := solver.kernel.internalizer.theories[family]
		_, legacy := plugin.(*SMTStringTheory)
		if legacy == initialAuto {
			t.Fatal("first push retained constructor-time QF_S backend", plugin)
		}
		astTestMust(t, solver.Pop(1))
		if solver.kernel.internalizer.theories[family] != plugin {
			t.Fatal("scope pop changed selected backend")
		}
		astTestMust(t, solver.Close())
		astTestMust(t, cfg.Context.Del(&params))
	}
}

func TestSMTGenericSelectorRespectsAutomaticSpecializedDispatch(t *testing.T) {
	for _, static := range []bool{false, true} {
		m, cfg := smtModelGeneratorFixture(t)
		params, err := cfg.Context.NewValue()
		astTestMust(t, err)
		key := "string_solver"
		astTestMust(t, cfg.Context.SetSymbolName(&params, &key, astTestSymbol(t, m, "invalid-selector")))
		solver, err := NewSMTSolver(m, &params, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
		astTestMust(t, err)
		astTestMust(t, solver.Assert(smtSetupCheckBound(t, m)))
		err = solver.kernel.setupAssertedBounds(static)
		if static {
			astTestMust(t, err) // setup_unknown(st) selects integer arithmetic.
		} else if err == nil || !strings.Contains(err.Error(), "invalid parameter for smt.string_solver") {
			t.Fatal("logic-only setup_unknown skipped generic selector validation", err)
		}
		astTestMust(t, solver.Close())
		astTestMust(t, cfg.Context.Del(&params))
	}
}

func TestSMTQFSSetupInstallsNativeStringAndMixedArithmetic(t *testing.T) {
	for _, profile := range []struct {
		auto   bool
		choice string
	}{
		{true, "seq"}, {true, "z3str3"}, {true, "auto"}, {true, "invalid-generic-choice"}, {false, "seq"},
	} {
		auto, choice := profile.auto, profile.choice
		t.Run(choice+map[bool]string{false: "-logic", true: "-auto"}[auto], func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			params, err := config.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, config.Context.Del(&params)) })
			key := "auto_config"
			astTestMust(t, config.Context.SetBoolName(&params, &key, auto))
			key = "string_solver"
			symbol := astTestSymbol(t, m, choice)
			astTestMust(t, config.Context.SetSymbolName(&params, &key, symbol))
			solver, err := NewSMTSolver(m, &params, "QF_S", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			seq, err := m.SequenceFamilyID()
			astTestMust(t, err)
			str, ok := solver.kernel.internalizer.theories[seq].(*SMTStringTheory)
			if auto && !ok {
				t.Fatal("QF_S profile must select native theory_str independently of generic string_solver")
			}
			if !auto {
				if _, isSeq := solver.kernel.internalizer.theories[seq].(*SMTSequenceTheory); !isSeq {
					t.Fatal("CFG_BASIC default seq selector did not retain generic sequence theory")
				}
			}
			arith, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			if arithmetic, ok := solver.kernel.internalizer.theories[arith].(*SMTTheoryArith); !ok || !(!arithmetic.integerExt) {
				t.Fatal("QF_S must select native theory_mi_arith")
			}
			if auto && (str.parameterOwner != &solver.kernel.smtParams.str || str.regexExpressionSolver == nil || str.regexExpressionSolver.kernel.smtParams != solver.kernel.smtParams) {
				t.Fatal("QF_S owner/nested kernel lost actual borrowed parameter ownership")
			}
		})
	}
}
