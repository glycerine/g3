package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDREngineRebuildsContextForChangedQuery(t *testing.T) {
	fixture := newPDRLifecycleFixture(t)
	status, err := fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal("initial PDR query", status, err)
	}
	first := fixture.engine.pdr
	status, err = fixture.engine.QueryPredicate(fixture.declarations["u"])
	if err != nil || status != lbool.False {
		t.Fatal("changed PDR query", status, err)
	}
	if fixture.engine.pdr == first {
		t.Fatal("changed PDR query reused its context")
	}
}
