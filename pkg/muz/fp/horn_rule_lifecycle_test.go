package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestHornRuleLifecycleSnapshotAndSubsumption(t *testing.T) {
	fixture := newPDRLifecycleFixture(t)
	snapshot, err := snapshotHornRules(fixture.engine.set)
	if err != nil {
		t.Fatal(err)
	}
	covered, err := hornRuleSetSubsumes(snapshot, fixture.engine.set)
	if err != nil || !covered {
		t.Fatal("snapshot did not cover its source", covered, err)
	}
	empty, err := base.NewRuleSet(fixture.engine.rules)
	if err != nil {
		t.Fatal(err)
	}
	covered, err = hornRuleSetSubsumes(empty, fixture.engine.set)
	if err != nil || covered {
		t.Fatal("empty old rules covered nonempty new rules", covered, err)
	}
	if err = empty.Close(); err != nil {
		t.Fatal(err)
	}
	if err = snapshot.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestHornRuleLifecycleFirstUncoveredRuleOrder(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	snapshot, err := snapshotHornRules(f.engine.set)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, snapshot.Close()) }()
	missing, err := firstUncoveredHornRule(snapshot, f.engine.set)
	fragmentMust(t, err)
	if missing != -1 {
		t.Fatal("covered source rule index", missing)
	}
	u, err := f.manager.MkApp(f.declarations["u"], nil)
	fragmentMust(t, err)
	fragmentMust(t, f.engine.addRuleEager(u, "first-fresh"))
	// A later fresh rule must not replace the first one in the source diagnostic.
	notP, err := f.manager.MkNot(f.applications["p"])
	fragmentMust(t, err)
	later, err := f.manager.MkImplies(notP, u)
	fragmentMust(t, err)
	fragmentMust(t, f.engine.addRuleEager(later, "later-fresh"))
	before := f.manager.Stats().LiveReferences
	missing, err = firstUncoveredHornRule(snapshot, f.engine.set)
	fragmentMust(t, err)
	if missing != 2 {
		t.Fatal("first fresh source rule index", missing)
	}
	if f.manager.Stats().LiveReferences != before {
		t.Fatal("subsumption scan leaked rule references")
	}
}
