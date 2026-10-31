package fp_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestTabEngineQueryProofAndSnapshots(t *testing.T) {
	engine, m := newRoutingEngine(t)
	fpMust(t, engine.SelectEngine("tab"))
	_, fact := routingPredicate(t, engine, m, "tab-fact")
	goalDecl, goal := routingPredicate(t, engine, m, "tab-goal")
	missingDecl, _ := routingPredicate(t, engine, m, "tab-missing")
	fpMust(t, engine.AddRule(fact, "fact"))
	step, err := m.MkImplies(fact, goal)
	fpMust(t, err)
	fpMust(t, engine.AddRule(step, "step"))
	status, err := engine.QueryPredicate(goalDecl)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("reachable query", status)
	}
	answer, err := engine.Answer(goalDecl)
	fpMust(t, err)
	proof, err := m.RefGet(answer)
	fpMust(t, err)
	kind, err := m.GetDeclKind(proof)
	fpMust(t, err)
	if kind != ast.ASTPrHyperResolve {
		t.Fatal("proof kind", kind)
	}
	conclusion, err := m.GetProofFact(proof)
	fpMust(t, err)
	if !m.IsFalse(conclusion) {
		t.Fatal("proof conclusion", conclusion)
	}
	fpMust(t, m.DelRef(&answer))
	status, err = engine.QueryPredicate(missingDecl)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("unreachable query", status)
	}
	answer, err = engine.Answer(missingDecl)
	fpMust(t, err)
	value, err := m.RefGet(answer)
	fpMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("unreachable answer")
	}
	fpMust(t, m.DelRef(&answer))
	status, err = engine.QueryExpression(goal)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("expression query", status)
	}
	answer, err = engine.Answer(goalDecl)
	fpMust(t, err)
	proof, err = m.RefGet(answer)
	fpMust(t, err)
	if _, err = m.GetProofFact(proof); err != nil {
		t.Fatal("clone proof did not survive", err)
	}
	fpMust(t, m.DelRef(&answer))
	fpMust(t, engine.AddRule(goal, "extra"))
	if _, err = engine.Answer(goalDecl); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("stale answer", err)
	}
}
