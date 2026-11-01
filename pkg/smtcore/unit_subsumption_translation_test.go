package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestUnitSubsumptionDirectTranslationUsesTargetServices(t *testing.T) {
	source, sourceServices := smtModelGeneratorFixture(t)
	target, targetServices := smtModelGeneratorFixture(t)
	targetConfig := SMTSolverConfig{Params: targetServices.Context, Globals: targetServices.Globals, ModelEvaluator: targetServices}
	sourceConfig := SMTSolverConfig{Params: sourceServices.Context, Globals: sourceServices.Globals, ModelEvaluator: sourceServices}
	sourceConfig.TargetConfig = func(manager *ast.ASTManager) (SMTSolverConfig, error) {
		if manager != target {
			t.Fatal("resolver received the wrong target")
		}
		return targetConfig, nil
	}
	setting, err := sourceServices.Context.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, sourceServices.Context.Del(&setting)) }()
	key := "random_seed"
	astTestMust(t, sourceServices.Context.SetUIntName(&setting, &key, 41))
	original, err := NewUnitSubsumptionTactic(source, &setting, sourceConfig)
	astTestMust(t, err)
	defer func() { astTestMust(t, original.Close()) }()
	if _, err = original.Translate(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil manager", err)
	}
	if _, err = original.TranslateWithConfig(target, SMTSolverConfig{}); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("missing target services", err)
	}
	original.config.TargetConfig = nil
	if _, err = original.Translate(target); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("missing target resolver", err)
	}
	refused := errors.New("target resolver refused")
	original.config.TargetConfig = func(*ast.ASTManager) (SMTSolverConfig, error) {
		return SMTSolverConfig{}, refused
	}
	if _, err = original.Translate(target); !errors.Is(err, refused) {
		t.Fatal("target resolver failure", err)
	}
	original.config.TargetConfig = sourceConfig.TargetConfig
	translated, err := original.Translate(target)
	astTestMust(t, err)
	copy := translated.(*UnitSubsumptionTactic)
	if copy.config.Params != targetServices.Context || copy.config.Globals != targetServices.Globals {
		t.Fatal("translated tactic did not use target parameter owners")
	}
	defer func() { astTestMust(t, copy.Close()) }()
	astTestMust(t, original.Close())
	seed, err := targetServices.Context.GetUIntName(&copy.params, &key, 0)
	astTestMust(t, err)
	if seed != 41 {
		t.Fatal("translated parameter", seed)
	}
	a := goalTestAtom(t, target, "translated-subsumption-a")
	b := goalTestAtom(t, target, "translated-subsumption-b")
	weak, err := target.MkOr(a, b)
	astTestMust(t, err)
	goal, err := tactic.NewGoal(target, false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	astTestMust(t, goal.Assert(weak))
	astTestMust(t, goal.Assert(a))
	var result tactic.TacticResult
	defer func() { astTestMust(t, result.Close()) }()
	astTestMust(t, copy.Apply(goal, &result))
	count, err := goal.Size()
	astTestMust(t, err)
	if count != 1 {
		t.Fatal("translated goal size", count)
	}
	form, err := goal.Form(0)
	astTestMust(t, err)
	if form != a {
		t.Fatal("translated surviving formula", form, a)
	}
}
