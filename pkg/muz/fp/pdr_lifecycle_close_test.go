package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDREngineCloseReleasesRetainedRuleSnapshot(t *testing.T) {
	fixture := newPDRLifecycleFixture(t)
	status, err := fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	snapshot := fixture.engine.pdrRules
	if snapshot == nil {
		t.Fatal("missing PDR rule snapshot")
	}
	if err = fixture.engine.Close(); err != nil {
		t.Fatal(err)
	}
	if fixture.engine.pdrRules != nil || fixture.engine.pdrRuleRevision != 0 {
		t.Fatal("PDR lifecycle state survived close")
	}
	if _, err = snapshot.Len(); err == nil {
		t.Fatal("retained PDR rule snapshot remained open")
	}
}
