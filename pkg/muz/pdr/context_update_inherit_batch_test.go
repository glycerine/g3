package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestContextUpdateRulesInheritsRetainedPredicateFrames(t *testing.T) {
	fixture := newValidationBatchFixture(t, "update_inherit_batch")
	property := pdrBool(t, fixture.manager, "update_inherited_property_batch")
	pdrMust(t, fixture.context.AddCover(1, fixture.declaration, property))
	set := updateBatchRuleSet(t, fixture, fixture.declaration)
	pdrMust(t, fixture.context.UpdateRules(set))
	pdrMust(t, set.Close())
	cover, err := fixture.context.CoverDelta(1, fixture.declaration)
	pdrMust(t, err)
	handle, err := fixture.manager.RefGet(cover)
	pdrMust(t, err)
	conjuncts, err := fixture.manager.FlattenAnd([]ast.ASTHandle{handle})
	pdrMust(t, err)
	found := false
	for _, conjunct := range conjuncts {
		found = found || conjunct == property
	}
	if !found {
		t.Fatal("updated transformer lost inherited frame", conjuncts)
	}
	pdrMust(t, fixture.manager.DelRef(&cover))
}
