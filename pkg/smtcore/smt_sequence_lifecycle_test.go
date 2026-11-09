package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTSequenceTranslationPreservesConfiguredBackendAndModels(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, configured := range []bool{false, true} {
			for _, sameManager := range []bool{false, true} {
				t.Run(fmt.Sprint(legacy, "/", configured, "/", sameManager), func(t *testing.T) {
					m, cfg := smtModelGeneratorFixture(t)
					target, targetCfg := m, cfg
					if !sameManager {
						target, targetCfg = smtModelGeneratorFixture(t)
					}
					params, err := cfg.Context.NewValue()
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
					targetParams, err := targetCfg.Context.NewValue()
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, targetCfg.Context.Del(&targetParams)) })
					key := "auto_config"
					astTestMust(t, cfg.Context.SetBoolName(&params, &key, false))
					astTestMust(t, targetCfg.Context.SetBoolName(&targetParams, &key, false))
					key = "string_solver"
					choices := map[bool]string{false: "seq", true: "z3str3"}
					astTestMust(t, cfg.Context.SetSymbolName(&params, &key, astTestSymbol(t, m, choices[legacy])))
					astTestMust(t, targetCfg.Context.SetSymbolName(&targetParams, &key, astTestSymbol(t, target, choices[!legacy])))
					s, err := NewSMTSolver(m, &params, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg, TargetConfig: func(manager *ast.ASTManager) (SMTSolverConfig, error) {
						if manager != target {
							return SMTSolverConfig{}, ast.ErrASTHandle
						}
						return SMTSolverConfig{Params: targetCfg.Context, ModelEvaluator: targetCfg}, nil
					}})
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, s.Close()) })
					if legacy {
						// Set the native public base member directly to
						// distinguish its reference from destination state.
						// smt_params constructor/update omit local String options.
						s.kernel.smtParams.str.state.StringConstantCache = false
					}
					astTestMust(t, s.SetProduceModels(true))
					x := astTestStringConst(t, m, "translated-sequence")
					literal := astTestString(t, m, "abc")
					equality, err := m.MkEq(x, literal)
					astTestMust(t, err)
					astTestMust(t, s.Assert(equality))
					if configured {
						status, err := s.CheckSat(nil)
						astTestMust(t, err)
						if status != lbool.True {
							t.Fatal("source sequence", status)
						}
					}
					value, err := s.Translate(target, &targetParams)
					astTestMust(t, err)
					dest := value.(*SMTSolver)
					t.Cleanup(func() { astTestMust(t, dest.Close()) })
					tx := x
					if !sameManager {
						translation, err := ast.NewASTTranslation(m, target, false)
						astTestMust(t, err)
						tx, err = translation.Translate(x)
						astTestMust(t, err)
						astTestPin(t, target, tx)
						astTestMust(t, translation.Close())
					}
					status, err := dest.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("translated sequence", status)
					}
					family, err := target.SequenceFamilyID()
					astTestMust(t, err)
					str, isLegacy := dest.kernel.internalizer.theories[family].(*SMTStringTheory)
					wantLegacy := legacy
					if !configured {
						wantLegacy = !legacy
					}
					if isLegacy != wantLegacy {
						t.Fatal("translation changed installed sequence backend", isLegacy, wantLegacy)
					}
					if isLegacy {
						owner := &dest.kernel.smtParams.str
						if configured {
							owner = &s.kernel.smtParams.str
						}
						if str.parameterOwner != owner || (str.regexExpressionSolver != nil && str.regexExpressionSolver.kernel.smtParams != dest.kernel.smtParams) {
							t.Fatal("String copy lost source String owner or destination nested-solver owner")
						}
					}
					generated, err := dest.AcquireModel()
					astTestMust(t, err)
					evaluator, err := model.NewModelEvaluator(generated, targetCfg)
					astTestMust(t, err)
					out, err := evaluator.EvaluateValue(tx)
					astTestMust(t, err)
					got, err := target.RefGet(out)
					astTestMust(t, err)
					text, matched, err := target.SequenceStringValue(got)
					astTestMust(t, err)
					if !matched || text != "abc" {
						t.Fatal("translated sequence model", text, matched)
					}
					astTestMust(t, target.DelRef(&out))
					astTestMust(t, evaluator.Close())
					astTestMust(t, generated.DecRef())
					if configured && legacy {
						// Compare the actual retained source base member with
						// the destination's independently initialized value.
						before := str.cacheAccesses
						_, err = str.stringLiteral("source-owner-cache-probe")
						astTestMust(t, err)
						if str.cacheAccesses != before || !dest.kernel.smtParams.str.state.StringConstantCache {
							t.Fatal("String copy used destination options instead of source constructor member")
						}
					}
					astTestMust(t, s.Close())
					chainedValue, err := dest.Translate(target, &targetParams)
					astTestMust(t, err)
					chained := chainedValue.(*SMTSolver)
					t.Cleanup(func() { astTestMust(t, chained.Close()) })
					if isLegacy {
						copy, ok := chained.kernel.internalizer.theories[family].(*SMTStringTheory)
						if !ok {
							t.Fatal("chained translation changed installed sequence backend")
						}
						if copy == str || copy.parameterOwner != str.parameterOwner || (copy.regexExpressionSolver != nil && copy.regexExpressionSolver.kernel.smtParams != chained.kernel.smtParams) || copy.NumVariables() != 0 || len(copy.strings) != 0 || len(copy.refs) != 0 {
							t.Fatal("chained String copy shared search state or changed parameter owners")
						}
					}
					astTestMust(t, dest.Close())
					if isLegacy {
						copy := chained.kernel.internalizer.theories[family].(*SMTStringTheory)
						truth, err := target.MkTrue()
						astTestMust(t, err)
						astTestMust(t, copy.Setup()) // theory_str::init at setup
						nested, err := copy.regexExpressionSolver.CheckSatisfiable(truth)
						astTestMust(t, err)
						if nested != lbool.True {
							t.Fatal("nested String solver borrowed closed source services", nested)
						}
					}
					status, err = chained.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("chained sequence lost retained parameters after source close", status)
					}
					astTestMust(t, chained.Push())
					empty := astTestString(t, target, "")
					conflict, err := target.MkEq(tx, empty)
					astTestMust(t, err)
					astTestMust(t, chained.Assert(conflict))
					status, err = chained.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.False {
						t.Fatal("chained sequence lost source assertion in child scope", status)
					}
					astTestMust(t, chained.Pop(1))
					status, err = chained.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("chained sequence failed parent recovery", status)
					}
				})
			}
		}
	}
}

func TestSMTSequenceAuxiliaryCopyKeepsInstalledBackendAndOwner(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprint(legacy), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			p, err := cfg.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&p)) })
			key := "auto_config"
			astTestMust(t, cfg.Context.SetBoolName(&p, &key, false))
			key = "string_solver"
			choice := "seq"
			if legacy {
				choice = "z3str3"
			}
			astTestMust(t, cfg.Context.SetSymbolName(&p, &key, astTestSymbol(t, m, choice)))
			s, err := NewSMTSolver(m, &p, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()) })
			x := astTestStringConst(t, m, "auxiliary-sequence")
			value := astTestString(t, m, "abc")
			eq, err := m.MkEq(x, value)
			astTestMust(t, err)
			astTestMust(t, s.Assert(eq))
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("source sequence", status)
			}
			a, err := s.newAuxiliarySolver()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, a.Close()) })
			family, err := m.SequenceFamilyID()
			astTestMust(t, err)
			plugin := a.kernel.internalizer.theories[family]
			str, isLegacy := plugin.(*SMTStringTheory)
			if isLegacy != legacy || plugin == s.kernel.internalizer.theories[family] || a.kernel.sequenceSetupPlaceholder != nil || plugin.(interface{ NumVariables() int }).NumVariables() != 0 {
				t.Fatal("auxiliary did not receive a fresh installed sequence plugin")
			}
			if isLegacy && (str.parameterOwner != &s.kernel.smtParams.str || (str.regexExpressionSolver != nil && str.regexExpressionSolver.kernel.smtParams != a.kernel.smtParams) || len(str.refs) != 0 || len(str.strings) != 0) {
				t.Fatal("auxiliary String owner or fresh state")
			}
			astTestMust(t, a.kernel.copySequencePlugin(s.kernel))
			if a.kernel.internalizer.theories[family] != plugin {
				t.Fatal("native registration replaced an already installed plugin")
			}
			// The copied plugin is already installed even though a has never
			// run setup. A second copy must not mistake it for a placeholder.
			b, err := a.newAuxiliarySolver()
			astTestMust(t, err)
			second := b.kernel.internalizer.theories[family]
			secondString, secondLegacy := second.(*SMTStringTheory)
			if secondLegacy != legacy || b.kernel.sequenceSetupPlaceholder != nil || second == plugin || second.(interface{ NumVariables() int }).NumVariables() != 0 || secondLegacy && secondString.parameterOwner != str.parameterOwner {
				t.Fatal("pre-setup auxiliary chain discarded an installed plugin")
			}
			astTestMust(t, b.Close())

			astTestMust(t, a.Assert(eq))
			status, err = a.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True || a.kernel.internalizer.theories[family] != plugin {
				t.Fatal("auxiliary setup replaced copied backend or lost query", status)
			}
			astTestMust(t, a.Close())
			status, err = s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("auxiliary close retired borrowed parent parameters", status)
			}
		})
	}
}
