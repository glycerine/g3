package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDREngineResetsContextForChangedQuery(t *testing.T) {
	fixture := newPDRLifecycleFixture(t)
	status, err := fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal("initial PDR query", status, err)
	}
	first := fixture.engine.pdr
	transformer, found, err := first.Transformer(fixture.engine.pdrPredicate(fixture.declarations["q"]))
	if err != nil || !found {
		t.Fatal("initial query transformer", found, err)
	}
	if err = transformer.EnsureLevel(2); err != nil {
		t.Fatal(err)
	}
	snapshot := fixture.engine.pdrSourceRules
	status, err = fixture.engine.QueryPredicate(fixture.declarations["u"])
	if err != nil || status != lbool.False {
		t.Fatal("changed PDR query", status, err)
	}
	if fixture.engine.pdr != first {
		t.Fatal("changed query replaced the source context instead of resetting it")
	}
	if _, err := transformer.Head(); err == nil {
		t.Fatal("changed query preserved its old transformer and frames")
	}
	if _, err := snapshot.Len(); err == nil {
		t.Fatal("changed query left its old source snapshot open")
	}
	if _, err := fixture.engine.Answer(fixture.declarations["q"]); err == nil {
		t.Fatal("changed query exposed the previous query's answer")
	}
	status, err = fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal("restored PDR query", status, err)
	}
	if fixture.engine.pdr != first {
		t.Fatal("restored query replaced the source context")
	}
	levels, err := first.NumLevels(fixture.engine.pdrPredicate(fixture.declarations["q"]))
	if err != nil || levels >= 3 {
		t.Fatal("restored query inherited frames from its earlier program", levels, err)
	}
}
