package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDREngineResetsContextForGeneralRuleAddition(t *testing.T) {
	fixture := newPDRLifecycleFixture(t)
	status, err := fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	first := fixture.engine.pdr
	transformer, found, err := first.Transformer(fixture.engine.pdrQueryPredicate(fixture.declarations["q"]))
	if err != nil || !found {
		t.Fatal("initial transformer", found, err)
	}
	if err = transformer.EnsureLevel(2); err != nil {
		t.Fatal(err)
	}
	if err = fixture.engine.addRuleEager(fixture.applications["u"], "pdr-lifecycle-extra"); err != nil {
		t.Fatal(err)
	}
	status, err = fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	if fixture.engine.pdr != first {
		t.Fatal("general rule addition replaced the source context instead of resetting it")
	}
	if _, err := transformer.Head(); err == nil {
		t.Fatal("uncovered addition left the old transformer open")
	}
	levels, err := first.NumLevels(fixture.engine.pdrQueryPredicate(fixture.declarations["q"]))
	if err != nil || levels >= 3 {
		t.Fatal("uncovered addition preserved the old frames", levels, err)
	}
}
