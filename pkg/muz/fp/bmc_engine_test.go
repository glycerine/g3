package fp_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBMCEngineRuleTraceAndSnapshot(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	fpMust(t, engine.SelectEngine("bmc"))
	_, fact := routingPredicate(t, engine, manager, "bmc-fact")
	goalDecl, goal := routingPredicate(t, engine, manager, "bmc-goal")
	fpMust(t, engine.AddRule(fact, "bmc-fact"))
	implication, err := manager.MkImplies(fact, goal)
	fpMust(t, err)
	fpMust(t, engine.AddRule(implication, "bmc-step"))
	status, err := engine.QueryPredicate(goalDecl)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("BMC query", status)
	}
	formulas, names, err := engine.RulesAlongTrace()
	fpMust(t, err)
	for i := range formulas {
		fpMust(t, manager.DelRef(&formulas[i]))
	}
	if len(names) != 2 || names[0] != "bmc-step" || names[1] != "bmc-fact" {
		t.Fatal("BMC trace", names)
	}
	if _, err := engine.Answer(goalDecl); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("unimplemented proof answer", err)
	}
	fpMust(t, engine.AddRule(goal, "extra"))
	if _, _, err = engine.RulesAlongTrace(); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("stale trace", err)
	}
}

func TestBMCEngineExpressionQueryKeepsTraceAfterCloneCloses(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	fpMust(t, engine.SelectEngine("bmc"))
	_, fact := routingPredicate(t, engine, manager, "bmc-expression-fact")
	_, goal := routingPredicate(t, engine, manager, "bmc-expression-goal")
	fpMust(t, engine.AddRule(fact, "expression-fact"))
	implication, err := manager.MkImplies(fact, goal)
	fpMust(t, err)
	fpMust(t, engine.AddRule(implication, "expression-step"))
	status, err := engine.QueryExpression(goal)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("expression query", status)
	}
	formulas, names, err := engine.RulesAlongTrace()
	fpMust(t, err)
	for i := range formulas {
		fpMust(t, manager.DelRef(&formulas[i]))
	}
	foundFact, foundStep := false, false
	for _, name := range names {
		foundFact = foundFact || name == "expression-fact"
		foundStep = foundStep || name == "expression-step"
	}
	if !foundFact || !foundStep {
		t.Fatal("cloned rule trace", names)
	}
}
