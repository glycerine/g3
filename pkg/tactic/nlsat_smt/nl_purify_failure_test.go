package nlsatsmt

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestNLPurifyApplyAllocationPrefixAndRetry(t *testing.T) {
	// Sweep this mixed goal through the first successful allocation frontier.
	// Other formulas and numerical interruption paths require separate audits.
	completed := false
	for stop := int64(0); stop < 2048 && !completed; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m := astTestManager(t)
			_, _, config := nlModelEvaluatorFixture(t, m)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			real, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			x := analysisConst(t, m, real, "failure-x")
			decl, err := m.MkFuncDecl(astTestSymbol(t, m, "failure-f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			h, err := m.MkApp(decl, []ast.ASTHandle{x})
			fx := analysisKeep(t, m, h, err)
			h, err = m.ArithmeticRealNumeralText("1")
			one := analysisKeep(t, m, h, err)
			h, err = m.MkArithmeticGE(fx, one)
			formula := analysisKeep(t, m, h, err)
			engine, err := NewNLPurifyTactic(m, config.Context, config.Globals, nil, config)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, engine.Close()) })
			newGoal := func() *tactic.Goal {
				g, e := tactic.NewGoal(m, false, true, false)
				astTestMust(t, e)
				t.Cleanup(func() { astTestMust(t, g.Close()) })
				astTestMust(t, g.Assert(formula))
				return g
			}
			var result tactic.TacticResult
			t.Cleanup(func() { astTestMust(t, result.Close()) })
			goal := newGoal()
			m.Budget().FailAfter(stop)
			err = engine.Apply(goal, &result)
			m.Budget().FailAfter(-1)
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				astTestMust(t, construction.Cleanup())
			}
			if err == nil {
				if !result.DecidedSat() || result.ModelConverter == nil {
					t.Fatal("successful search lacks a model")
				}
				t.Logf("first successful allocation frontier: %d", stop)
				completed = true
				return
			}
			var failure memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &failure) && failure.Kind == memory.FailureMemout) {
				t.Fatal(err)
			}
			astTestMust(t, engine.Cleanup())
			retry := newGoal()
			var retried tactic.TacticResult
			t.Cleanup(func() { astTestMust(t, retried.Close()) })
			astTestMust(t, engine.Apply(retry, &retried))
			if !retried.DecidedSat() || retried.ModelConverter == nil {
				t.Fatal("mixed goal failed after cleanup retry")
			}
		})
	}
	if !completed {
		t.Fatal("allocation sweep did not reach success")
	}
}

func TestNLPurifyCleanupAllocationFrontiers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 2048 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m := astTestManager(t)
			_, _, config := nlModelEvaluatorFixture(t, m)
			engine, err := NewNLPurifyTactic(m, config.Context, config.Globals, nil, config)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, engine.Close()) })
			m.Budget().FailAfter(stop)
			err = engine.Cleanup()
			m.Budget().FailAfter(-1)
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				astTestMust(t, construction.Cleanup())
			}
			if err == nil {
				success = true
			} else {
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &failure) && failure.Kind == memory.FailureMemout) {
					t.Fatal(err)
				}
				astTestMust(t, engine.Cleanup())
			}
			goal, err := tactic.NewGoal(m, false, true, false)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, goal.Close()) })
			truth, err := m.MkTrue()
			astTestMust(t, err)
			astTestMust(t, goal.Assert(truth))
			var result tactic.TacticResult
			t.Cleanup(func() { astTestMust(t, result.Close()) })
			astTestMust(t, engine.Apply(goal, &result))
			if !result.DecidedSat() || result.ModelConverter == nil {
				t.Fatal("cleanup retry did not produce a satisfiable result")
			}
		})
	}
	if !success {
		t.Fatal("no successful cleanup allocation frontier")
	}
}

func TestNLPurifyConstructorAllocationFrontiers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 2048 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m := astTestManager(t)
			_, _, config := nlModelEvaluatorFixture(t, m)
			// The source global registry deliberately does not retry a partial
			// first registration, so initialize it before testing tactic owners.
			_, err := config.Globals.SourceGetValueName("auto_config")
			astTestMust(t, err)
			m.Budget().FailAfter(stop)
			engine, err := NewNLPurifyTactic(m, config.Context, config.Globals, nil, config)
			m.Budget().FailAfter(-1)
			if engine != nil {
				astTestMust(t, engine.Close())
			}
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				astTestMust(t, construction.Cleanup())
			}
			if err == nil {
				success = true
				return
			}
			var failure memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &failure) && failure.Kind == memory.FailureMemout) {
				t.Fatal(err)
			}
			retry, err := NewNLPurifyTactic(m, config.Context, config.Globals, nil, config)
			astTestMust(t, err)
			astTestMust(t, retry.Close())
		})
	}
	if !success {
		t.Fatal("no successful constructor allocation frontier")
	}
}

func TestNLPurifyAllocationOtherModes(t *testing.T) {
	for _, tc := range []struct {
		name         string
		cores, unsat bool
	}{
		{"sat-cores", true, false}, {"unsat", false, true}, {"unsat-cores", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			completed := false
			for stop := int64(0); stop < 2048 && !completed; stop++ {
				if !t.Run(fmt.Sprint(stop), func(t *testing.T) {
					m := astTestManager(t)
					_, _, config := nlModelEvaluatorFixture(t, m)
					family, err := m.RegisterArithmeticPlugin()
					astTestMust(t, err)
					real, err := m.MkFamilySort(family, 0, nil)
					astTestMust(t, err)
					x := analysisConst(t, m, real, "other-failure-x")
					decl, err := m.MkFuncDecl(astTestSymbol(t, m, "other-failure-f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
					astTestMust(t, err)
					h, err := m.MkApp(decl, []ast.ASTHandle{x})
					fx := analysisKeep(t, m, h, err)
					h, err = m.ArithmeticRealNumeralText("1")
					one := analysisKeep(t, m, h, err)
					h, err = m.ArithmeticRealNumeralText("0")
					zero := analysisKeep(t, m, h, err)
					h, err = m.MkArithmeticGE(fx, one)
					positive := analysisKeep(t, m, h, err)
					h, err = m.MkArithmeticLE(fx, zero)
					negative := analysisKeep(t, m, h, err)
					boolSort, err := m.MkBoolSort()
					astTestMust(t, err)
					labels := []ast.ASTHandle{analysisConst(t, m, boolSort, "other-failure-label-1"), analysisConst(t, m, boolSort, "other-failure-label-2")}
					engine, err := NewNLPurifyTactic(m, config.Context, config.Globals, nil, config)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, engine.Close()) })
					newGoal := func() *tactic.Goal {
						g, e := tactic.NewGoal(m, false, true, tc.cores)
						astTestMust(t, e)
						t.Cleanup(func() { astTestMust(t, g.Close()) })
						forms := []ast.ASTHandle{positive}
						if tc.unsat {
							forms = append(forms, negative)
						}
						for i, form := range forms {
							var dep ast.ASTDependency
							if tc.cores {
								dep, e = m.DependencyLeaf(labels[i])
								astTestMust(t, e)
							}
							astTestMust(t, g.AssertWithProof(form, ast.ASTHandle{}, dep))
						}
						return g
					}
					check := func(result *tactic.TacticResult) {
						if tc.unsat {
							if !result.DecidedUnsat() {
								t.Fatal("expected unsat")
							}
							if tc.cores {
								leaves, e := m.DependencyLinearize(result.Core, nil)
								astTestMust(t, e)
								if len(leaves) != 2 || !((leaves[0] == labels[0] && leaves[1] == labels[1]) || (leaves[0] == labels[1] && leaves[1] == labels[0])) {
									t.Fatal("core leaves", leaves, labels)
								}
							}
						} else {
							if !result.DecidedSat() || result.ModelConverter == nil {
								t.Fatal("expected sat with model")
							}
							var answer tactic.ModelRef
							astTestMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &answer))
							defer func() { astTestMust(t, answer.Close()) }()
							core, e := answer.Get()
							astTestMust(t, e)
							evaluator, e := model.NewModelEvaluator(core, config)
							astTestMust(t, e)
							defer func() { astTestMust(t, evaluator.Close()) }()
							value, e := evaluator.EvaluateValue(positive)
							astTestMust(t, e)
							defer func() { astTestMust(t, m.DelRef(&value)) }()
							actual, e := m.RefGet(value)
							astTestMust(t, e)
							if !m.IsTrue(actual) {
								t.Fatal("model does not satisfy input", actual)
							}
						}
					}
					var result tactic.TacticResult
					t.Cleanup(func() { astTestMust(t, result.Close()) })
					goal := newGoal()
					m.Budget().FailAfter(stop)
					err = engine.Apply(goal, &result)
					m.Budget().FailAfter(-1)
					var construction memory.ConstructionFailure
					if errors.As(err, &construction) {
						astTestMust(t, construction.Cleanup())
					}
					if err == nil {
						check(&result)
						t.Logf("success frontier %d", stop)
						completed = true
						return
					}
					var failure memory.Failure
					if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &failure) && failure.Kind == memory.FailureMemout) {
						t.Fatal(err)
					}
					astTestMust(t, engine.Cleanup())
					retry := newGoal()
					var retried tactic.TacticResult
					t.Cleanup(func() { astTestMust(t, retried.Close()) })
					astTestMust(t, engine.Apply(retry, &retried))
					check(&retried)
				}) {
					return
				}
			}
			if !completed {
				t.Fatal("did not reach successful frontier")
			}
		})
	}
}
