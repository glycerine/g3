package base_test

import "testing"

func TestRuleCopyAssignsNameAndClonePreservesName(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	source, err := f.rules.NewRule(f.head, nil, nil, "source-name")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, source.Close()) })
	copy, err := f.rules.CopyRule(source, "copy-name")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, copy.Close()) })
	if name, err := copy.Name(); err != nil || name != "copy-name" {
		t.Fatal("copy name", name, err)
	}
	clone, err := f.rules.CloneRule(source)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, clone.Close()) })
	if name, err := clone.Name(); err != nil || name != "source-name" {
		t.Fatal("clone name", name, err)
	}
}
