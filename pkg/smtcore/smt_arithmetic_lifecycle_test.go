package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtArithmeticLifecycleKind(theory SMTTheoryInternalizer) string {
	switch value := theory.(type) {
	case *SMTLRATheory:
		return "lra"
	case *SMTLinearArithmeticTheory:
		if value.infinite {
			return "infinite-simplex"
		}
		if value.mixed {
			return "mixed-simplex"
		}
		if value.integer {
			return "integer-simplex"
		}
		return "real-simplex"
	case *SMTTheoryArith:
		if value.infExt {
			return "infinite-simplex"
		}
		if value.integerExt {
			return "integer-simplex"
		}
		return "mixed-simplex"
	case *SMTRealDiffTheory:
		if value.mixed {
			return "mixed-graph"
		}
		return "real-graph"
	case *SMTDenseDiffTheory:
		return "dense"
	case *SMTDummyTheory:
		return "dummy"
	default:
		return "other"
	}
}

func TestSMTArithmeticTranslationRetainsConcreteBackendsAndModels(t *testing.T) {
	for _, sourceCase := range []struct {
		kind, logic        string
		mode               uint32
		automatic, integer bool
	}{
		{"mixed-simplex", "ALL", 2, false, true},
		{"infinite-simplex", "ALL", 5, false, true},
		{"integer-simplex", "QF_LIA", 2, true, true},
		{"lra", "QF_LRA", 2, true, false},
		{"mixed-graph", "ALL", 1, false, true},
		{"real-graph", "QF_RDL", 2, true, false},
		{"dense", "ALL", 3, false, true},
		{"dummy", "ALL", 0, false, true},
	} {
		for _, targetMode := range []uint32{0, 2, 4} {
			t.Run(fmt.Sprint(sourceCase.kind, "/", targetMode), func(t *testing.T) {
				m, cfg := smtModelGeneratorFixture(t)
				target, targetCfg := smtModelGeneratorFixture(t)
				params, err := cfg.Context.NewValue()
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
				name := "auto_config"
				astTestMust(t, cfg.Context.SetBoolName(&params, &name, sourceCase.automatic))
				name = "arith.solver"
				astTestMust(t, cfg.Context.SetUIntName(&params, &name, sourceCase.mode))
				targetParams, err := targetCfg.Context.NewValue()
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, targetCfg.Context.Del(&targetParams)) })
				name = "auto_config"
				astTestMust(t, targetCfg.Context.SetBoolName(&targetParams, &name, false))
				name = "arith.solver"
				astTestMust(t, targetCfg.Context.SetUIntName(&targetParams, &name, targetMode))
				config := SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg, TargetConfig: func(manager *ast.ASTManager) (SMTSolverConfig, error) {
					if manager != target {
						return SMTSolverConfig{}, ast.ErrASTHandle
					}
					return SMTSolverConfig{Params: targetCfg.Context, ModelEvaluator: targetCfg}, nil
				}}
				s, err := NewSMTSolver(m, &params, sourceCase.logic, config)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, s.Close()) })
				astTestMust(t, s.SetProduceModels(true))
				family, err := m.ArithmeticFamilyID()
				astTestMust(t, err)
				kind := int32(0)
				if sourceCase.integer {
					kind = 1
				}
				sort, err := m.MkFamilySort(family, kind, nil)
				astTestMust(t, err)
				x, err := m.MkConst(astTestSymbol(t, m, "arithmetic-clone-x"), sort)
				astTestMust(t, err)
				for _, bound := range []struct {
					op    int32
					value int64
				}{{smtArithOpGE, 1}, {smtArithOpLE, 2}} {
					n, err := m.ArithmeticNumeralRat(big.NewRat(bound.value, 1), sourceCase.integer)
					astTestMust(t, err)
					formula, err := m.MkFamilyApp(family, bound.op, nil, []ast.ASTHandle{x, n})
					astTestMust(t, err)
					astTestMust(t, s.Assert(formula))
				}
				if sourceCase.kind == "real-graph" {
					// This retained RDL route needs Boolean-rich sparse features.
					boolean := smtRelevancyBool(t, m, "arithmetic-clone-p")
					astTestMust(t, s.Assert(boolean))
				}
				// Select each source backend through the static setup overload;
				// subsequent translated and original checks are incremental.
				status, err := smtSetupProfileCheck(s)
				astTestMust(t, err)
				want := lbool.True
				if sourceCase.kind == "dummy" {
					want = lbool.Undef
				}
				if status != want {
					t.Fatal("source status", status)
				}
				original := s.kernel.internalizer.theories[family]
				if actual := smtArithmeticLifecycleKind(original); actual != sourceCase.kind {
					t.Fatal("source case did not reach its retained backend", actual)
				}
				value, err := s.Translate(target, &targetParams)
				astTestMust(t, err)
				translated := value.(*SMTSolver)
				t.Cleanup(func() { astTestMust(t, translated.Close()) })
				status, err = translated.CheckSat(nil)
				astTestMust(t, err)
				if status != want {
					t.Fatal("copied source backend was replaced", status, want)
				}
				targetFamily, err := target.ArithmeticFamilyID()
				astTestMust(t, err)
				copied := translated.kernel.internalizer.theories[targetFamily]
				if copied == original || smtArithmeticLifecycleKind(copied) != sourceCase.kind {
					t.Fatal("concrete backend was not freshly copied", smtArithmeticLifecycleKind(copied))
				}
				if want == lbool.Undef {
					return
				}
				translation, err := ast.NewASTTranslation(m, target, false)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, translation.Close()) })
				targetX, err := translation.Translate(x)
				astTestMust(t, err)
				astTestMust(t, translated.Push())
				zero, err := target.ArithmeticNumeralRat(new(big.Rat), sourceCase.integer)
				astTestMust(t, err)
				conflict, err := target.MkFamilyApp(targetFamily, smtArithOpLE, nil, []ast.ASTHandle{targetX, zero})
				astTestMust(t, err)
				astTestMust(t, translated.Assert(conflict))
				status, err = translated.CheckSat(nil)
				astTestMust(t, err)
				if status != lbool.False {
					t.Fatal("copied child contradiction", status)
				}
				astTestMust(t, translated.Pop(1))
				status, err = translated.CheckSat(nil)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("copied parent recovery", status)
				}
				generated, err := translated.AcquireModel()
				astTestMust(t, err)
				decl, err := target.GetDecl(targetX)
				astTestMust(t, err)
				modelValue, err := generated.ConstantInterpretation(decl)
				astTestMust(t, err)
				text, integer, matched, err := target.ArithmeticNumeralText(modelValue)
				astTestMust(t, err)
				astTestMust(t, generated.DecRef())
				number, parsed := new(big.Rat).SetString(text)
				if !matched || !parsed || integer != sourceCase.integer || number.Cmp(big.NewRat(1, 1)) < 0 || number.Cmp(big.NewRat(2, 1)) > 0 {
					t.Fatal("copied model bounds or sort", text, integer)
				}
				status, err = s.CheckSat(nil)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("translated scopes changed the source", status)
				}
			})
		}
	}
}

func TestSMTArithmeticTranslationRetainsLiveSourceParameterOwner(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	target, targetCfg := smtModelGeneratorFixture(t)
	params, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
	name := "auto_config"
	astTestMust(t, cfg.Context.SetBoolName(&params, &name, false))
	name = "arith.branch_cut_ratio"
	astTestMust(t, cfg.Context.SetUIntName(&params, &name, 7))
	config := SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg, TargetConfig: func(manager *ast.ASTManager) (SMTSolverConfig, error) {
		return SMTSolverConfig{Params: targetCfg.Context, ModelEvaluator: targetCfg}, nil
	}}
	s, err := NewSMTSolver(m, &params, "ALL", config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	n, err := m.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	astTestMust(t, err)
	sort, err := m.GetSort(n)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "live-clone-x"), sort)
	astTestMust(t, err)
	bound, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, n})
	astTestMust(t, err)
	astTestMust(t, s.Assert(bound))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	targetParams, err := targetCfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, targetCfg.Context.Del(&targetParams)) })
	name = "auto_config"
	astTestMust(t, targetCfg.Context.SetBoolName(&targetParams, &name, false))
	name = "arith.branch_cut_ratio"
	astTestMust(t, targetCfg.Context.SetUIntName(&targetParams, &name, 19))
	value, err := s.Translate(target, &targetParams)
	astTestMust(t, err)
	translated := value.(*SMTSolver)
	t.Cleanup(func() { astTestMust(t, translated.Close()) })
	check := func(want uint32) {
		status, err := translated.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal(status)
		}
		family, err := target.ArithmeticFamilyID()
		astTestMust(t, err)
		copied := translated.kernel.internalizer.theories[family].(*SMTLinearArithmeticTheory)
		if copied.branchCutRatio != want {
			t.Fatal("fresh plugin did not use its source parameter owner", copied.branchCutRatio, want)
		}
	}
	check(7)
	name = "arith.branch_cut_ratio"
	astTestMust(t, cfg.Context.SetUIntName(&params, &name, 11))
	astTestMust(t, s.UpdateParams(&params))
	astTestMust(t, translated.Push())
	check(11)
	astTestMust(t, translated.Pop(1))
	astTestMust(t, targetCfg.Context.SetUIntName(&targetParams, &name, 23))
	astTestMust(t, translated.UpdateParams(&targetParams))
	check(11)
	// Managed parameter state remains readable when the source solver closes;
	// the copy does not depend on a live source tableau, model, or AST manager.
	astTestMust(t, s.Close())
	astTestMust(t, m.Close())
	check(11)
	value, err = translated.Translate(target, &targetParams)
	astTestMust(t, err)
	again := value.(*SMTSolver)
	t.Cleanup(func() { astTestMust(t, again.Close()) })
	status, err = again.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || again.kernel.copiedArithmeticOwner != translated.kernel.copiedArithmeticOwner {
		t.Fatal("chained translation lost the original arithmetic parameter owner", status)
	}
	targetFamily, err := target.ArithmeticFamilyID()
	astTestMust(t, err)
	if again.kernel.internalizer.theories[targetFamily].(*SMTLinearArithmeticTheory).branchCutRatio != 11 {
		t.Fatal("chained translation used the intermediate context's parameter values")
	}
}

func TestSMTArithmeticTranslationBeforeSetupUsesDestinationSelection(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	params, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
	name := "auto_config"
	astTestMust(t, cfg.Context.SetBoolName(&params, &name, false))
	name = "arith.solver"
	astTestMust(t, cfg.Context.SetUIntName(&params, &name, 4))
	s, err := NewSMTSolver(m, &params, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	one, err := m.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	astTestMust(t, err)
	sort, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "pre-setup-clone-x"), sort)
	astTestMust(t, err)
	formula, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, one})
	astTestMust(t, err)
	astTestMust(t, s.Assert(formula))
	targetParams, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&targetParams)) })
	name = "auto_config"
	astTestMust(t, cfg.Context.SetBoolName(&targetParams, &name, false))
	name = "arith.solver"
	astTestMust(t, cfg.Context.SetUIntName(&targetParams, &name, 3))
	value, err := s.Translate(m, &targetParams)
	astTestMust(t, err)
	translated := value.(*SMTSolver)
	t.Cleanup(func() { astTestMust(t, translated.Close()) })
	if translated.kernel.copiedArithmeticPlugin {
		t.Fatal("unconfigured Go placeholder was copied as an installed source plugin")
	}
	status, err := translated.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	if _, ok := translated.kernel.internalizer.theories[family].(*SMTDenseDiffTheory); !ok {
		t.Fatal("unconfigured source prevented destination mode selection")
	}
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	if _, ok := s.kernel.internalizer.theories[family].(*SMTUTVPITheory); !ok {
		t.Fatal("translation changed source pre-setup mode")
	}
}
