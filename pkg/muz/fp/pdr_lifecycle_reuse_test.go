package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDREngineReusesContextForStrengthenedNamedRule(t *testing.T) {
	fixture := newPDRLifecycleFixture(t)
	status, err := fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.True {
		t.Fatal("initial PDR query", status, err)
	}
	firstContext, firstRules := fixture.engine.pdr, fixture.engine.pdrRules
	if firstContext == nil || firstRules == nil {
		t.Fatal("PDR lifecycle state was not retained")
	}
	strongBody, err := fixture.manager.MkAnd(fixture.applications["p"], fixture.applications["u"])
	if err != nil {
		t.Fatal(err)
	}
	strongEdge, err := fixture.manager.MkImplies(strongBody, fixture.applications["q"])
	if err == nil {
		err = fixture.engine.UpdateRule(strongEdge, "pdr-lifecycle-edge")
	}
	if err != nil {
		t.Fatal(err)
	}
	status, err = fixture.engine.QueryPredicate(fixture.declarations["q"])
	if err != nil || status != lbool.False {
		t.Fatal("strengthened PDR query", status, err)
	}
	if fixture.engine.pdr != firstContext || fixture.engine.pdrRules == firstRules {
		t.Fatal("PDR did not reuse context and rotate transformed rules")
	}
}
