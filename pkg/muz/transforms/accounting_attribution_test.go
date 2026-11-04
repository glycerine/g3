package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func checkTransformAccounting(t *testing.T, transformed, original *base.Rule, delta base.Costs) {
	t.Helper()
	before, err := original.AccountingCosts()
	coneMust(t, err)
	charge := base.NewAccountedObject()
	defer func() { coneMust(t, charge.Close()) }()
	coneMust(t, charge.SetAccountingParentRule(transformed))
	costs, err := charge.CurrentCosts()
	coneMust(t, err)
	*costs = delta
	coneMust(t, charge.ProcessCosts())
	got, err := original.AccountingCosts()
	coneMust(t, err)
	before.Add(delta)
	if got != before {
		t.Fatalf("original rule accounting = %+v, want %+v", got, before)
	}
}
