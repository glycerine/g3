package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

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
