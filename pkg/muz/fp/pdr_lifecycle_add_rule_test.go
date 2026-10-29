package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDREngineRebuildsContextForGeneralRuleAddition(t *testing.T) {
	fixture := newPDRLifecycleFixture(t)
	status, err := fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	first := fixture.engine.pdr
	if err = fixture.engine.AddRule(fixture.applications["u"], "pdr-lifecycle-extra"); err != nil {
		t.Fatal(err)
	}
	status, err = fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	if fixture.engine.pdr == first {
		t.Fatal("general rule addition reused a PDR context")
	}
}
