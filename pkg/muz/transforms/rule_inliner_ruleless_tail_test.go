package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

// inlining_allowed holds for a tail predicate without rules (m_head_pred_ctr
// 0): plan_inlining's transform_rule inlines Q :- E to nothing and deletes it
// with a false body (del_rule(r0, true)), so m_inlined_rules is empty;
// transform_rules then deletes Goal :- Q (false body) and the rules of the
// eliminated Q.
func TestRuleInlinerInlinesRulelessTailPredicate(t *testing.T) {
	f := newConeFixture(t, "E", "Q", "Goal")
	f.edge(t, "E", "Q")
	f.edge(t, "Q", "Goal")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("inliner chain rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	plugin := transforms.NewRuleInlinerPlugin(f.manager, f.rules, nil, 0)
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("rules over the rule-less predicate were not inlined")
	}
	if count, err := f.set.Len(); err != nil || count != 0 {
		t.Fatal("inlined rule count", count, err)
	}
	modelRules, err := plugin.TakeModelRules()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, transforms.CloseInlineModelRules(modelRules)) })
	want := []string{"Q", "Goal", "Q"}
	if len(modelRules) != len(want) {
		t.Fatal("inliner model-rule count", len(modelRules))
	}
	for i, name := range want {
		head, err := f.manager.RefGet(modelRules[i].Head)
		coneMust(t, err)
		decl, err := f.manager.GetDecl(head)
		coneMust(t, err)
		body, err := f.manager.RefGet(modelRules[i].Body)
		coneMust(t, err)
		if decl != f.declarations[name] || !f.manager.IsFalse(body) {
			t.Fatal("model rule", i, name, decl == f.declarations[name], f.manager.IsFalse(body))
		}
	}
}
