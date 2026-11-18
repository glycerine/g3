package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTUTVPITranslationPreservesInstalledPluginAndIndependentState(t *testing.T) {
	for _, sameManager := range []bool{false, true} {
		for _, targetMode := range []uint32{0, 2, 4} {
			t.Run(fmt.Sprint(sameManager, "/", targetMode), func(t *testing.T) {
				m, cfg := smtModelGeneratorFixture(t)
				target, targetCfg := m, cfg
				if !sameManager {
					target, targetCfg = smtModelGeneratorFixture(t)
				}
				params, err := cfg.Context.NewValue()
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
				name := "auto_config"
				astTestMust(t, cfg.Context.SetBoolName(&params, &name, false))
				name = "arith.solver"
				astTestMust(t, cfg.Context.SetUIntName(&params, &name, 4))
				targetParams, err := targetCfg.Context.NewValue()
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, targetCfg.Context.Del(&targetParams)) })
				name = "auto_config"
				astTestMust(t, targetCfg.Context.SetBoolName(&targetParams, &name, false))
				name = "arith.solver"
				astTestMust(t, targetCfg.Context.SetUIntName(&targetParams, &name, targetMode))
				config := SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg}
				config.TargetConfig = func(manager *ast.ASTManager) (SMTSolverConfig, error) {
					if manager != target {
						return SMTSolverConfig{}, ast.ErrASTHandle
					}
					return SMTSolverConfig{Params: targetCfg.Context, ModelEvaluator: targetCfg}, nil
				}
				s, err := NewSMTSolver(m, &params, "ALL", config)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, s.Close()) })
				astTestMust(t, s.SetProduceModels(true))
				family, err := m.ArithmeticFamilyID()
				astTestMust(t, err)
				sort, err := m.MkFamilySort(family, 1, nil)
				astTestMust(t, err)
				x, err := m.MkConst(astTestSymbol(t, m, "utvpi-translate-x"), sort)
				astTestMust(t, err)
				for _, bound := range []struct {
					op    int32
					value int64
				}{{smtArithOpGE, 1}, {smtArithOpLE, 2}} {
					n, err := bigTestArithNumeral(m, big.NewRat(bound.value, 1), true)
					astTestMust(t, err)
					formula, err := m.MkFamilyApp(family, bound.op, nil, []ast.ASTHandle{x, n})
					astTestMust(t, err)
					astTestMust(t, s.Assert(formula))
				}
				status, err := s.CheckSat(nil)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("source UTVPI model", status)
				}
				sourceTheory := s.kernel.internalizer.theories[family].(*SMTUTVPITheory)
				sourceCounts := sourceTheory.statistics
				value, err := s.Translate(target, &targetParams)
				astTestMust(t, err)
				translated := value.(*SMTSolver)
				t.Cleanup(func() { astTestMust(t, translated.Close()) })
				status, err = translated.CheckSat(nil)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("translation discarded the installed source UTVPI plugin", status)
				}
				targetFamily, err := target.ArithmeticFamilyID()
				astTestMust(t, err)
				copied, ok := translated.kernel.internalizer.theories[targetFamily].(*SMTUTVPITheory)
				if !ok {
					t.Fatal("translation reselected arithmetic from destination parameters")
				}
				if copied == sourceTheory || copied.graph == sourceTheory.graph || copied.graph.graph == sourceTheory.graph.graph || copied.adapter == sourceTheory.adapter {
					t.Fatal("fresh-context UTVPI state aliases the source")
				}
				targetX := x
				if !sameManager {
					translation, err := ast.NewASTTranslation(m, target, false)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, translation.Close()) })
					targetX, err = translation.Translate(x)
					astTestMust(t, err)
				}
				astTestMust(t, translated.Push())
				zero, err := bigTestArithNumeral(target, new(big.Rat), true)
				astTestMust(t, err)
				conflict, err := target.MkFamilyApp(targetFamily, smtArithOpLE, nil, []ast.ASTHandle{targetX, zero})
				astTestMust(t, err)
				astTestMust(t, translated.Assert(conflict))
				status, err = translated.CheckSat(nil)
				astTestMust(t, err)
				if status != lbool.False {
					t.Fatal("translated child contradiction", status)
				}
				astTestMust(t, translated.Pop(1))
				status, err = translated.CheckSat(nil)
				astTestMust(t, err)
				if status != lbool.True || sourceTheory.statistics != sourceCounts || s.kernel.Inconsistent() {
					t.Fatal("translated scopes changed source state or failed to recover")
				}
				generated, err := translated.AcquireModel()
				astTestMust(t, err)
				declaration, err := target.GetDecl(targetX)
				astTestMust(t, err)
				modelValue, err := generated.ConstantInterpretation(declaration)
				astTestMust(t, err)
				text, integer, matched, err := target.ArithmeticNumeralText(modelValue)
				astTestMust(t, err)
				astTestMust(t, generated.DecRef())
				number, parsed := new(big.Rat).SetString(text)
				if !integer || !matched || !parsed || number.Cmp(big.NewRat(1, 1)) < 0 || number.Cmp(big.NewRat(2, 1)) > 0 {
					t.Fatal("translated model violated copied bounds", text)
				}
			})
		}
	}
}
