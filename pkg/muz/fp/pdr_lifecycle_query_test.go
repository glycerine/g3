package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDREngineResetsContextForRepeatedQuery(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr"} {
		t.Run(backend, func(t *testing.T) {
			fixture := newPDRLifecycleFixture(t)
			if err := fixture.engine.SelectEngine(backend); err != nil {
				t.Fatal(err)
			}
			query := fixture.declarations["q"]
			status, err := fixture.engine.QueryPredicate(query)
			if err != nil || status != lbool.True {
				t.Fatal("initial PDR query", status, err)
			}
			context := fixture.engine.pdr
			transformer, found, err := context.Transformer(fixture.engine.pdrPredicate(query))
			if err != nil || !found {
				t.Fatal("initial query transformer", found, err)
			}
			if err = transformer.EnsureLevel(2); err != nil {
				t.Fatal(err)
			}
			transformed, source := fixture.engine.pdrRules, fixture.engine.pdrSourceRules
			status, err = fixture.engine.QueryPredicate(query)
			if err != nil || status != lbool.True {
				t.Fatal("repeated PDR query", status, err)
			}
			if fixture.engine.pdr != context {
				t.Fatal("repeated query replaced the configured source context")
			}
			if _, err = transformer.Head(); err == nil {
				t.Fatal("fresh synthetic query rule preserved the old transformer")
			}
			if _, err = transformed.Len(); err == nil {
				t.Fatal("repeated query left the transformed snapshot open")
			}
			if _, err = source.Len(); err == nil {
				t.Fatal("repeated query left the source snapshot open")
			}
			installed, found, err := context.Transformer(fixture.engine.pdrPredicate(query))
			if err != nil || !found || installed == transformer {
				t.Fatal("repeated query did not install a fresh transformer", found, err)
			}
			levels, err := installed.NumLevels()
			if err != nil || levels >= 3 {
				t.Fatal("repeated query inherited old frames", levels, err)
			}
		})
	}
}

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
