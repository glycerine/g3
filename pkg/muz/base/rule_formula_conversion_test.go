package base_test

import "testing"

func TestRuleManagerRuleFormulaAndReceiverFormula(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	rule, err := f.rules.NewRule(f.head, nil, nil, "formula-conversion")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	managed, err := f.rules.RuleFormula(rule)
	ruleMust(t, err)
	receiver, err := rule.Formula()
	ruleMust(t, err)
	managedHandle, err := f.manager.RefGet(managed)
	ruleMust(t, err)
	receiverHandle, err := f.manager.RefGet(receiver)
	ruleMust(t, err)
	if managedHandle != receiverHandle {
		t.Fatal("formula conversion is not canonical")
	}
	ruleMust(t, f.manager.DelRef(&receiver))
	ruleMust(t, f.manager.DelRef(&managed))
}
