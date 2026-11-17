package base_test

// Source: src/muz/base/dl_rule_set.cpp rule_set::add_rule
// (m_head2rules.insert_if_not_there2), close/reopen/reset (m_deps.reset(),
// reset_dealloc_values) and src/util/hashtable.h core_hashtable's
// insert_if_not_there_core and reset.

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

// TestRuleSetTableCapacities: add_rule's insert_if_not_there2 checks the
// load before looking for the head, so a rule for a present head expands a
// table holding 7 of 8 entries; reopen and replace_rules reset m_deps and
// m_head2rules without shrinking them, so the next close populates m_deps
// in a table of the earlier capacity (its iteration order is the
// compiler's detect_chains order).
func TestRuleSetTableCapacities(t *testing.T) {
	names := []string{"A", "B", "C", "D", "E", "F", "G"}
	f := newRuleSetFixture(t, names...)
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	add := func(s *base.RuleSet, head string, tail ...string) {
		r := f.rule(head, tail, nil)
		ruleMust(t, s.Add(r))
		ruleMust(t, r.Close())
	}
	for i, name := range names {
		if i+1 < len(names) {
			add(set, name, names[i+1])
		} else {
			add(set, name)
		}
	}
	if head, _ := set.TableCapacitiesForTest(); head != 8 {
		t.Fatal("7 heads in m_head2rules", head)
	}
	add(set, "A", "C")
	if head, _ := set.TableCapacitiesForTest(); head != 16 {
		t.Fatal("a present head's insert_if_not_there2 expands", head)
	}
	if ok, err := set.Finalize(); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if _, deps := set.TableCapacitiesForTest(); deps != 16 {
		t.Fatal("m_deps of 7 predicates", deps)
	}
	small, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, small.Close()) })
	add(small, "A", "B")
	add(small, "B")
	ruleMust(t, set.Reopen())
	ruleMust(t, set.ReplaceRules(small))
	if ok, err := set.Finalize(); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if head, deps := set.TableCapacitiesForTest(); head != 16 || deps != 16 {
		t.Fatal("reset keeps the capacities", head, deps)
	}
}
