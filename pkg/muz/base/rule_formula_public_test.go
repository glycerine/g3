package base_test

import (
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleFormulaPublicOwnershipAndClosedFailures(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	rule, err := f.rules.NewRule(f.head, nil, nil, "formula-public")
	ruleMust(t, err)
	if err = f.rules.WriteRuleFormula(nil, rule); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("nil writer", err)
	}
	ruleMust(t, rule.Close())
	if _, err = rule.FormulaString(); !errors.Is(err, base.ErrRuleOwner) {
		t.Fatal("closed rule accepted", err)
	}
}
