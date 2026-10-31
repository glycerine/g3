package fp_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
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
	// Default inlining folds both named rules into the synthetic query rule.
	if len(names) != 1 || names[0] != "" {
		t.Fatal("BMC trace", names)
	}
	answer, err := engine.Answer(goalDecl)
	fpMust(t, err)
	proof, err := manager.RefGet(answer)
	fpMust(t, err)
	conclusion, err := manager.GetProofFact(proof)
	fpMust(t, err)
	declaration, err := manager.GetDecl(conclusion)
	fpMust(t, err)
	name, err := manager.GetASTName(declaration)
	fpMust(t, err)
	text, err := manager.Symbols.Display(name)
	fpMust(t, err)
	if !strings.HasPrefix(text, "query!") {
		t.Fatal("BMC proof conclusion", text)
	}
	fpMust(t, manager.DelRef(&answer))
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
	// The synthetic query can absorb both named rules during preprocessing.
	if len(formulas) == 0 || len(formulas) != len(names) {
		t.Fatal("cloned rule trace", names)
	}
	answer, err := engine.Answer(ast.ASTHandle{})
	fpMust(t, err)
	proof, err := manager.RefGet(answer)
	fpMust(t, err)
	if _, err = manager.GetProofFact(proof); err != nil {
		t.Fatal("cloned proof is not live", err)
	}
	fpMust(t, manager.DelRef(&answer))
}
