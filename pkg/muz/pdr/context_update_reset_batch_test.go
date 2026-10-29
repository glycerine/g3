package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestContextUpdateRulesResetsPriorSolveCertificate(t *testing.T) {
	fixture := newValidationBatchFixture(t, "update_reset_batch")
	status, err := fixture.context.Solve(fixture.declaration)
	pdrMust(t, err)
	if status != lbool.True {
		t.Fatal("pre-update solve", status)
	}
	set := updateBatchRuleSet(t, fixture, fixture.declaration)
	pdrMust(t, fixture.context.UpdateRules(set))
	pdrMust(t, set.Close())
	status, err = fixture.context.LastResult()
	pdrMust(t, err)
	if status != lbool.Undef {
		t.Fatal("update retained stale result", status)
	}
}
