package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTArrayTranslationBackendsOwnersModelsAndScopes(t *testing.T) {
	for _, simple := range []bool{false, true} {
		for _, configured := range []bool{false, true} {
			for _, sameManager := range []bool{false, true} {
				t.Run(fmt.Sprint(simple, "/", configured, "/", sameManager), func(t *testing.T) {
					m, cfg := smtModelGeneratorFixture(t)
					target, targetCfg := m, cfg
					if !sameManager {
						target, targetCfg = smtModelGeneratorFixture(t)
					}
					params, err := cfg.Context.NewValue()
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
					name := "auto_config"
					astTestMust(t, cfg.Context.SetBoolName(&params, &name, simple))
					name = "array.extensional"
					astTestMust(t, cfg.Context.SetBoolName(&params, &name, false))
					targetParams, err := targetCfg.Context.NewValue()
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, targetCfg.Context.Del(&targetParams)) })
					name = "auto_config"
					astTestMust(t, targetCfg.Context.SetBoolName(&targetParams, &name, !simple))
					s, err := NewSMTSolver(m, &params, "QF_AX", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg, TargetConfig: func(manager *ast.ASTManager) (SMTSolverConfig, error) {
						if manager != target {
							return SMTSolverConfig{}, ast.ErrASTHandle
						}
						return SMTSolverConfig{Params: targetCfg.Context, ModelEvaluator: targetCfg}, nil
					}})
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, s.Close()) })
					astTestMust(t, s.SetProduceModels(true))
					family, err := m.RegisterArrayPlugin()
					astTestMust(t, err)
					boolean, err := m.MkBoolSort()
					astTestMust(t, err)
					sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
					astTestMust(t, err)
					a, err := m.MkConst(astTestSymbol(t, m, "copy-array-a"), sort)
					astTestMust(t, err)
					b, err := m.MkConst(astTestSymbol(t, m, "copy-array-b"), sort)
					astTestMust(t, err)
					astTestPin(t, m, a)
					astTestPin(t, m, b)
					index, err := m.MkTrue()
					astTestMust(t, err)
					read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, index})
					astTestMust(t, err)
					astTestMust(t, s.Assert(read))
					if configured {
						status, err := s.CheckSat(nil)
						astTestMust(t, err)
						if status != lbool.True {
							t.Fatal("source array", status)
						}
					}
					value, err := s.Translate(target, &targetParams)
					astTestMust(t, err)
					dest := value.(*SMTSolver)
					t.Cleanup(func() { astTestMust(t, dest.Close()) })
					translate := func(term ast.ASTHandle) (ast.ASTHandle, error) { return term, nil }
					if !sameManager {
						translation, err := ast.NewASTTranslation(m, target, false)
						astTestMust(t, err)
						t.Cleanup(func() { astTestMust(t, translation.Close()) })
						translate = translation.Translate
					}
					ta, err := translate(a)
					astTestMust(t, err)
					tb, err := translate(b)
					astTestMust(t, err)
					tread, err := translate(read)
					astTestMust(t, err)
					astTestPin(t, target, ta)
					astTestPin(t, target, tb)
					astTestPin(t, target, tread)
					status, err := dest.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("translated array", status)
					}
					targetFamily, err := target.ArrayFamilyID()
					astTestMust(t, err)
					core := smtArrayCore(dest.kernel.internalizer.theories[targetFamily])
					wantSimple := simple
					if !configured {
						wantSimple = !simple
					}
					if core == nil || core.simple != wantSimple || core == smtArrayCore(s.kernel.internalizer.theories[family]) {
						t.Fatal("translation did not preserve fresh concrete array backend")
					}
					// A configured full backend borrows source params. Simple
					// and pre-setup translations use destination params.
					borrowSource := configured && !simple
					probe := func(wantExtensional bool) {
						t.Helper()
						astTestMust(t, dest.Push())
						eq, err := target.MkEq(ta, tb)
						astTestMust(t, err)
						neq, err := target.MkNot(eq)
						astTestMust(t, err)
						astTestMust(t, dest.Assert(neq))
						for _, truth := range []bool{false, true} {
							index, err := target.MkFalse()
							if truth {
								index, err = target.MkTrue()
							}
							astTestMust(t, err)
							left, err := target.MkFamilyApp(targetFamily, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{ta, index})
							astTestMust(t, err)
							right, err := target.MkFamilyApp(targetFamily, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{tb, index})
							astTestMust(t, err)
							same, err := target.MkEq(left, right)
							astTestMust(t, err)
							astTestMust(t, dest.Assert(same))
						}
						status, err := dest.CheckSat(nil)
						astTestMust(t, err)
						want := lbool.True
						if wantExtensional {
							want = lbool.False
						}
						if status != want || core.extensional != wantExtensional {
							t.Fatal("translated parameter owner", status, want, core.extensional)
						}
						astTestMust(t, dest.Pop(1))
						status, err = dest.CheckSat(nil)
						astTestMust(t, err)
						if status != lbool.True {
							t.Fatal("translated parent recovery", status)
						}
					}
					probe(!borrowSource)
					name = "array.extensional"
					astTestMust(t, cfg.Context.SetBoolName(&params, &name, true))
					astTestMust(t, s.UpdateParams(&params))
					probe(true)
					astTestMust(t, targetCfg.Context.SetBoolName(&targetParams, &name, false))
					astTestMust(t, dest.UpdateParams(&targetParams))
					probe(borrowSource)
					astTestMust(t, s.Close())
					probe(borrowSource)
					chainedValue, err := dest.Translate(target, &targetParams)
					astTestMust(t, err)
					chained := chainedValue.(*SMTSolver)
					t.Cleanup(func() { astTestMust(t, chained.Close()) })
					status, err = chained.CheckSat(nil)
					astTestMust(t, err)
					chainedCore := smtArrayCore(chained.kernel.internalizer.theories[targetFamily])
					var wantOwner *SMTArrayParams
					if !core.simple {
						wantOwner = dest.kernel.copiedArrayOwner
						if wantOwner == nil {
							wantOwner = &dest.kernel.smtParams.array
						}
					}
					if status != lbool.True || chainedCore == core || chainedCore.simple != core.simple || chainedCore.extensional != borrowSource || chained.kernel.copiedArrayOwner != wantOwner {
						t.Fatal("chained translation lost its concrete backend or original parameter owner")
					}
					astTestMust(t, chained.Push())
					notRead, err := target.MkNot(tread)
					astTestMust(t, err)
					astTestMust(t, chained.Assert(notRead))
					status, err = chained.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.False {
						t.Fatal("chained array scope lost source assertion", status)
					}
					astTestMust(t, chained.Pop(1))
					generated, err := dest.AcquireModel()
					astTestMust(t, err)
					evaluator, err := model.NewModelEvaluator(generated, targetCfg)
					astTestMust(t, err)
					out, err := evaluator.EvaluateValue(tread)
					astTestMust(t, err)
					result, err := target.RefGet(out)
					astTestMust(t, err)
					if !target.IsTrue(result) {
						t.Fatal("translated model lost asserted array read")
					}
					astTestMust(t, target.DelRef(&out))
					astTestMust(t, evaluator.Close())
					astTestMust(t, generated.DecRef())
				})
			}
		}
	}
}

func TestSMTArrayTranslationInitializationUpdatesCorrectParameterOwner(t *testing.T) {
	for _, simple := range []bool{false, true} {
		m, cfg := smtModelGeneratorFixture(t)
		params, err := cfg.Context.NewValue()
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
		name := "auto_config"
		astTestMust(t, cfg.Context.SetBoolName(&params, &name, simple))
		source, err := NewSMTSolver(m, &params, "QF_AX", SMTSolverConfig{Params: cfg.Context})
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, source.Close()) })
		// The first-push profile retains relevance, unlike the unit-only
		// static first-check profile, so source init leaves laziness at 1.
		astTestMust(t, source.Push())
		astTestMust(t, source.Pop(1))
		if source.kernel.smtParams.array.state.Laziness != 1 {
			t.Fatal("source fixture did not retain lazy parents")
		}
		targetParams, err := cfg.Context.NewValue()
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&targetParams)) })
		name = "auto_config"
		astTestMust(t, cfg.Context.SetBoolName(&targetParams, &name, false))
		name = "relevancy"
		astTestMust(t, cfg.Context.SetUIntName(&targetParams, &name, 0))
		translated, err := source.Translate(m, &targetParams)
		astTestMust(t, err)
		dest := translated.(*SMTSolver)
		t.Cleanup(func() { astTestMust(t, dest.Close()) })
		family, err := m.ArrayFamilyID()
		astTestMust(t, err)
		core := smtArrayCore(dest.kernel.internalizer.theories[family])
		// context::copy registers the copied plugins on a fresh context whose
		// relevancy level is still 2 (fork backport of upstream z3 5dfe4a4b4),
		// so theory_array::init keeps lazy registration despite relevancy 0
		// in the target parameters.
		if core.simple != simple || core.arrayConfig.Laziness == 0 {
			t.Fatal("copied init disabled lazy registration before setup")
		}
		// The copied init no longer disables laziness, so the source owner
		// (shared by the full array theory) keeps lazy registration too.
		wantSourceLazy := uint32(1)
		if source.kernel.smtParams.array.state.Laziness != wantSourceLazy {
			t.Fatal("copied init mutated incorrect owner", simple)
		}
		name = "array.weak"
		astTestMust(t, cfg.Context.SetBoolName(&params, &name, true))
		astTestMust(t, source.UpdateParams(&params))
		status, err := dest.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True || core.arrayConfig.Weak == simple {
			t.Fatal("copied weak control did not follow its source/destination owner")
		}
		status, err = source.CheckSat(nil)
		astTestMust(t, err)
		original := smtArrayCore(source.kernel.internalizer.theories[family])
		if status != lbool.True || original.arrayConfig.Laziness != wantSourceLazy {
			t.Fatal("original backend missed shared parameter mutation")
		}
	}
}
