package base_test

import (
	"bytes"
	"strings"
	"testing"
)

func TestRuleManagerWriteRuleFormulaAndString(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	rule, err := f.rules.NewRule(f.head, nil, nil, "formula-display")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	var out bytes.Buffer
	ruleMust(t, f.rules.WriteRuleFormula(&out, rule))
	text, err := f.rules.RuleFormulaString(rule)
	ruleMust(t, err)
	if out.String() != text || !strings.Contains(text, "forall") || !strings.Contains(text, "analysis-P") {
		t.Fatal("rule display", text)
	}
}
