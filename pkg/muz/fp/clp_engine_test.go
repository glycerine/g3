package fp_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCLPEngineResultSnapshotAndAnswer(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	fpMust(t, engine.SelectEngine("clp"))
	declaration, application := routingPredicate(t, engine, manager, "clp")
	applicationRef, err := manager.NewRef(application)
	fpMust(t, err)
	defer func() { fpMust(t, manager.DelRef(&applicationRef)) }()
	status, err := engine.QueryPredicate(declaration)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("empty predicate", status)
	}
	answer, err := engine.Answer(declaration)
	fpMust(t, err)
	value, err := manager.RefGet(answer)
	fpMust(t, err)
	if !manager.IsTrue(value) {
		t.Fatal("CLP answer", value)
	}
	fpMust(t, manager.DelRef(&answer))
	fpMust(t, engine.AddRule(application, "fact"))
	if _, err = engine.Answer(declaration); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("stale answer", err)
	}
	status, err = engine.QueryPredicate(declaration)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("fact query", status)
	}
	if last, err := engine.LastResult(); err != nil || last != lbool.True {
		t.Fatal("result snapshot", last, err)
	}
	answer, err = engine.Answer(declaration)
	fpMust(t, err)
	fpMust(t, manager.DelRef(&answer))
}
