package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDRRuleUpdateDoesNotHideBackgroundRevision(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	query := f.declarations["q"]
	status, err := f.engine.QueryPredicate(query)
	must(err)
	if status != lbool.True {
		t.Fatal("initial rule must reach q", status)
	}
	prior := f.engine.pdr
	falsity, err := f.manager.MkFalse()
	must(err)
	must(f.engine.SetBackground(falsity))
	edge, err := f.manager.MkImplies(f.applications["p"], f.applications["q"])
	must(err)
	must(f.engine.UpdateRule(edge, "pdr-lifecycle-edge"))
	status, err = f.engine.QueryPredicate(query)
	must(err)
	if status != lbool.False {
		t.Fatal("rule update hid the false background", status)
	}
	if f.engine.pdr == prior {
		t.Fatal("rule update reused a context with the old background")
	}
	prior = f.engine.pdr
	truth, err := f.manager.MkTrue()
	must(err)
	must(f.engine.SetBackground(truth))
	must(f.engine.UpdateRule(edge, "pdr-lifecycle-edge"))
	status, err = f.engine.QueryPredicate(query)
	must(err)
	if status != lbool.True {
		t.Fatal("rule update hid the restored true background", status)
	}
	if f.engine.pdr == prior {
		t.Fatal("rule update reused the context with a false background")
	}
}

func TestPDRRuleUpdateDoesNotHideParameterRevision(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	query := f.declarations["q"]
	status, err := f.engine.QueryPredicate(query)
	must(err)
	if status != lbool.True {
		t.Fatal("initial rule must reach q", status)
	}
	prior := f.engine.pdr
	parameters, err := f.engine.params.NewValue()
	must(err)
	defer func() { must(f.engine.params.Del(&parameters)) }()
	name := "pdr.cache_mode"
	must(f.engine.params.SetUIntName(&parameters, &name, 2))
	must(f.engine.SetQueryParameters(&parameters))
	edge, err := f.manager.MkImplies(f.applications["p"], f.applications["q"])
	must(err)
	must(f.engine.UpdateRule(edge, "pdr-lifecycle-edge"))
	status, err = f.engine.QueryPredicate(query)
	must(err)
	if status != lbool.True {
		t.Fatal("parameter update changed q reachability", status)
	}
	if f.engine.pdr == prior {
		t.Fatal("rule update reused a context with old constructor parameters")
	}
	if _, err = prior.NumLevels(query); err == nil {
		t.Fatal("replaced parameter context remained open")
	}
}

func TestPDREngineRebuildsContextForBackgroundChange(t *testing.T) {
	fixture := newPDRLifecycleFixture(t)
	status, err := fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	first := fixture.engine.pdr
	truth, err := fixture.manager.MkTrue()
	if err == nil {
		err = fixture.engine.SetBackground(truth)
	}
	if err != nil {
		t.Fatal(err)
	}
	// Setting true is a no-op, so use an independent Boolean constraint.
	if err = fixture.engine.SetBackground(fixture.applications["u"]); err != nil {
		t.Fatal(err)
	}
	status, err = fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	if fixture.engine.pdr == first {
		t.Fatal("background change reused a PDR context")
	}
}
