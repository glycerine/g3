package core

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestCollectStatisticsAccumulatesAcrossApplyAndReset(t *testing.T) {
	f := newCollectStatisticsFixture(t)
	r := f.runner(t)
	term := f.app(t, f.f, f.a)
	applyStatistics(t, r, f.goal(t, term))
	simplifyMust(t, r.ResetStatistics())
	applyStatistics(t, r, f.goal(t, term))
	s, e := r.Snapshot()
	simplifyMust(t, e)
	if s["function-applications"] != 8 || s["uninterpreted-function-occurrences"] != 2 {
		t.Fatal(s)
	}
}

func TestCollectStatisticsRejectsForeignParameterOwner(t *testing.T) {
	manager, context, _ := simplifyFixture(t)
	foreign, err := uparams.NewParamsContext(manager.Symbols, manager.NumeralService(), uparams.ParamsConfig{Budget: manager.Budget()})
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, foreign.Close()) })
	params, err := foreign.NewValue()
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, foreign.Del(&params)) })
	if _, err = NewCollectStatisticsTacticWithServices(manager, context, &params, &bytes.Buffer{}); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatalf("foreign constructor parameters: %v", err)
	}
	runner, err := NewCollectStatisticsTacticWithServices(manager, context, nil, &bytes.Buffer{})
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	if err = runner.UpdateParams(&params); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatalf("foreign update parameters: %v", err)
	}
	if err = context.Check(&runner.params); err != nil {
		t.Fatal("foreign update damaged previous parameters", err)
	}
}

func TestCollectStatisticsTranslationHasFreshCountersAndSameOutput(t *testing.T) {
	f := newCollectStatisticsFixture(t)
	r := f.runner(t)
	applyStatistics(t, r, f.goal(t, f.a))
	translated, e := r.Translate(f.m)
	simplifyMust(t, e)
	copy := translated.(*CollectStatisticsTactic)
	t.Cleanup(func() { simplifyMust(t, copy.Close()) })
	s, e := copy.Snapshot()
	simplifyMust(t, e)
	if len(s) != 0 {
		t.Fatal("translated source owner copied counters", s)
	}
	if copy.output != r.output {
		t.Fatal("translation did not preserve selected stream")
	}
}

func TestCollectStatisticsCopiesAndTranslatesParameters(t *testing.T) {
	manager, context, _ := simplifyFixture(t)
	key := "source-setting"
	initial, err := context.NewValue()
	simplifyMust(t, err)
	simplifyMust(t, context.SetBoolName(&initial, &key, true))
	runner, err := NewCollectStatisticsTacticWithOutput(manager, &initial, &bytes.Buffer{})
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	simplifyMust(t, context.SetBoolName(&initial, &key, false))
	simplifyMust(t, context.Del(&initial))
	if value, err := context.GetBoolName(&runner.params, &key, false); err != nil || !value {
		t.Fatal("constructor did not keep parameter", value, err)
	}
	updated, err := context.NewValue()
	simplifyMust(t, err)
	simplifyMust(t, context.SetBoolName(&updated, &key, false))
	simplifyMust(t, runner.UpdateParams(&updated))
	simplifyMust(t, context.Del(&updated))
	if value, err := context.GetBoolName(&runner.params, &key, true); err != nil || value {
		t.Fatal("update did not keep replacement parameter", value, err)
	}
	translated, err := runner.Translate(manager)
	simplifyMust(t, err)
	copy := translated.(*CollectStatisticsTactic)
	t.Cleanup(func() { simplifyMust(t, copy.Close()) })
	simplifyMust(t, runner.Close())
	if value, err := context.GetBoolName(&copy.params, &key, true); err != nil || value {
		t.Fatal("translation did not own source parameter", value, err)
	}
}

func TestCollectStatisticsTranslationAcrossManagers(t *testing.T) {
	source, context, _ := simplifyFixture(t)
	params, err := context.NewValue()
	simplifyMust(t, err)
	key := "translation-parameter"
	simplifyMust(t, context.SetBoolName(&params, &key, true))
	var output bytes.Buffer
	runner, err := NewCollectStatisticsTacticWithServices(source, context, &params, &output)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	simplifyMust(t, context.Del(&params))
	target, _, _ := simplifyFixture(t)
	translated, err := runner.Translate(target)
	simplifyMust(t, err)
	copy := translated.(*CollectStatisticsTactic)
	t.Cleanup(func() { simplifyMust(t, copy.Close()) })
	simplifyMust(t, runner.Close())
	if value, err := context.GetBoolName(&copy.params, &key, false); err != nil || !value {
		t.Fatal("translated tactic lost its parameter", value, err)
	}
	goal, err := tactic.NewGoal(target, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	truth, err := target.MkTrue()
	simplifyMust(t, err)
	simplifyMust(t, goal.Assert(truth))
	result := &tactic.TacticResult{}
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, copy.Apply(goal, result))
	if output.Len() == 0 {
		t.Fatal("translated tactic did not write output")
	}
}

func TestCollectStatisticsContextTranslationUsesTargetParameters(t *testing.T) {
	source, sourceContext, _ := simplifyFixture(t)
	params, err := sourceContext.NewValue()
	simplifyMust(t, err)
	key := "translated-parameter"
	simplifyMust(t, sourceContext.SetBoolName(&params, &key, true))
	runner, err := NewCollectStatisticsTacticWithServices(source, sourceContext, &params, &bytes.Buffer{})
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	simplifyMust(t, sourceContext.Del(&params))
	target, targetContext, targetGlobals := simplifyFixture(t)
	translated, matched, err := TranslateContextLeaf(runner, target, targetContext, targetGlobals, model.ModelEvaluatorConfig{})
	simplifyMust(t, err)
	if !matched {
		t.Fatal("collect-statistics was not translated")
	}
	copy := translated.(*CollectStatisticsTactic)
	t.Cleanup(func() { simplifyMust(t, copy.Close()) })
	if copy.context != targetContext {
		t.Fatal("translated tactic uses source parameter owner")
	}
	simplifyMust(t, runner.Close())
	if value, err := targetContext.GetBoolName(&copy.params, &key, false); err != nil || !value {
		t.Fatal("translated parameter", value, err)
	}
}

func TestCollectStatisticsContextTranslationWithoutSourceParameterOwner(t *testing.T) {
	manager, context, globals := simplifyFixture(t)
	previous, err := uparams.BindProcessGlobalParamsService(nil)
	simplifyMust(t, err)
	runner, constructErr := NewCollectStatisticsTacticWithOutput(manager, nil, &bytes.Buffer{})
	_, restoreErr := uparams.BindProcessGlobalParamsService(previous)
	simplifyMust(t, restoreErr)
	simplifyMust(t, constructErr)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	if runner.paramsReady {
		t.Fatal("plain manager unexpectedly acquired parameters")
	}
	translated, matched, err := TranslateContextLeaf(runner, manager, context, globals, model.ModelEvaluatorConfig{})
	simplifyMust(t, err)
	if !matched {
		t.Fatal("collect-statistics was not translated")
	}
	copy := translated.(*CollectStatisticsTactic)
	t.Cleanup(func() { simplifyMust(t, copy.Close()) })
	if copy.context != context || !copy.paramsReady {
		t.Fatal("target parameter owner was not attached")
	}
}
