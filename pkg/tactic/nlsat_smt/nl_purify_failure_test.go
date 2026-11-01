package nlsatsmt

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestNLPurifyApplyAllocationPrefixAndRetry(t *testing.T) {
	// This prefix reaches polynomial compilation. Later constructor and
	// search interruption paths require their own ownership audit.
	for stop := int64(0); stop < 98; stop++ {
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
				t.Fatal("allocation prefix unexpectedly completed")
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
