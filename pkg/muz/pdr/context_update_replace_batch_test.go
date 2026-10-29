package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestContextUpdateRulesReplacesDeclarationSetAndRemainsSolvable(t *testing.T) {
	fixture := newValidationBatchFixture(t, "update_old_declaration_batch")
	boolean, err := fixture.manager.MkBoolSort()
	pdrMust(t, err)
	replacement := pdrFunction(t, fixture.manager, "update_new_declaration_batch", nil, boolean)
	set := updateBatchRuleSet(t, fixture, replacement)
	pdrMust(t, fixture.context.UpdateRules(set))
	pdrMust(t, set.Close())
	if _, found, x := fixture.context.Transformer(fixture.declaration); x != nil || found {
		t.Fatal("removed declaration survived update", found, x)
	}
	if _, found, x := fixture.context.Transformer(replacement); x != nil || !found {
		t.Fatal("replacement declaration missing", found, x)
	}
	status, err := fixture.context.Solve(replacement)
	pdrMust(t, err)
	if status != lbool.True {
		t.Fatal("replacement fact solve", status)
	}
}
