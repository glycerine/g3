package pdr_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestContextUpdateRulesRejectsProofModeChangeBeforeMutation(t *testing.T) {
	fixture := newValidationBatchFixture(t, "update_proof_guard_batch")
	arithmetic, err := fixture.manager.ArithmeticFamilyID()
	pdrMust(t, err)
	integer, err := fixture.manager.MkFamilySort(arithmetic, 1, nil)
	pdrMust(t, err)
	variable, err := fixture.manager.MkVar(0, integer)
	pdrMust(t, err)
	zero, err := fixture.manager.ArithmeticIntegerNumeralText("0")
	pdrMust(t, err)
	bound, err := fixture.manager.MkArithmeticLE(variable, zero)
	pdrMust(t, err)
	set := updateBatchRuleSet(t, fixture, fixture.declaration, bound)
	if err = fixture.context.UpdateRules(set); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("proof-mode-changing update", err)
	}
	pdrMust(t, set.Close())
	if _, found, err := fixture.context.Transformer(fixture.declaration); err != nil || !found {
		t.Fatal("rejected update mutated old transformer", found, err)
	}
}
