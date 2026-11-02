package fp_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDualityFixedpointDispatchProofModelScopesAndSwitching(t *testing.T) {
	e, m := newRoutingEngine(t)
	fpMust(t, e.SelectEngine("duality"))
	pd, p := routingPredicate(t, e, m, "duality-dispatch-P")
	qd, q := routingPredicate(t, e, m, "duality-dispatch-Q")
	fpMust(t, e.AddRule(p, "fact"))
	status, err := e.QueryPredicate(pd)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("reachable relation", status)
	}
	proof, err := e.Proof(pd)
	fpMust(t, err)
	defer func() { fpMust(t, m.DelRef(&proof)) }()
	h, err := m.RefGet(proof)
	fpMust(t, err)
	conclusion, err := m.GetProofFact(h)
	fpMust(t, err)
	if !m.IsFalse(conclusion) {
		t.Fatal("query proof must conclude false")
	}
	if _, err := e.Answer(pd); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("unsupported source get_answer", err)
	}
	status, err = e.QueryExpression(q)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("unreachable expression", status)
	}
	md, err := e.Model(qd)
	fpMust(t, err)
	defer func() { fpMust(t, md.Close()) }()
	fpMust(t, e.Push())
	fpMust(t, e.AddRule(q, "scoped-fact"))
	status, err = e.QueryPredicate(qd)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("scoped fact", status)
	}
	fpMust(t, e.Pop())
	if _, err := e.Proof(qd); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("pop left stale proof", err)
	}
	status, err = e.QueryPredicate(qd)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("popped fact survived", status)
	}
	fpMust(t, e.SelectEngine("pdr"))
	if _, err := e.Model(qd); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("switch left stale model", err)
	}
	fpMust(t, e.SelectEngine("duality"))
	status, err = e.QueryPredicate(pd)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("reselected backend", status)
	}
	fpMust(t, e.Close())
	if _, err := m.RefGet(proof); err != nil {
		t.Fatal("owned proof expired after switching/closure", err)
	}
}

func TestDualityFixedpointBackgroundAssertionsAndPop(t *testing.T) {
	e, m := newRoutingEngine(t)
	fpMust(t, e.SelectEngine("duality"))
	pd, p := routingPredicate(t, e, m, "duality-background-P")
	fpMust(t, e.AddRule(p, "fact"))
	fpMust(t, e.Push())
	falsity, err := m.MkFalse()
	fpMust(t, err)
	fpMust(t, e.AssertBackground(falsity))
	status, err := e.QueryPredicate(pd)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("inconsistent background must block reachability", status)
	}
	fpMust(t, e.Pop())
	status, err = e.QueryPredicate(pd)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("popped background still asserted", status)
	}
}
