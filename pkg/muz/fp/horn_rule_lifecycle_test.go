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
