package base_test

// Source: src/muz/base/dl_rule.{h,cpp},
// src/muz/base/dl_rule_subsumption_index.{h,cpp}, dl_context::check_subsumes,
// and PDR/Spacer dl_interface::check_reset.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
)

type subsumptionFixture struct {
	t       *testing.T
	context *cmd_context.Context
	manager *ast.ASTManager
	rules   *base.RuleManager
	p, q, r ast.ASTHandle
}

func newSubsumptionFixture(t *testing.T) *subsumptionFixture {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	manager, err := context.Manager()
	ruleMust(t, err)
	rules, err := base.NewRuleManager(manager)
	ruleMust(t, err)
	boolean, err := manager.MkBoolSort()
	ruleMust(t, err)
	applications := make([]ast.ASTHandle, 3)
	for i, suffix := range []string{"P", "Q", "R"} {
		name, x := manager.Symbols.Intern("subsumption-" + suffix)
		ruleMust(t, x)
		declaration, x := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, x)
		ruleMust(t, rules.RegisterPredicate(declaration))
		applications[i], x = manager.MkApp(declaration, nil)
		ruleMust(t, x)
	}
	fixture := &subsumptionFixture{t: t, context: context, manager: manager, rules: rules, p: applications[0], q: applications[1], r: applications[2]}
	t.Cleanup(func() {
		ruleMust(t, rules.Close())
		ruleMust(t, context.Close())
	})
	return fixture
}

func (f *subsumptionFixture) rule(name string, head ast.ASTHandle, tails []ast.ASTHandle, negative []bool) *base.Rule {
	f.t.Helper()
	rule, err := f.rules.NewRule(head, tails, negative, name)
	ruleMust(f.t, err)
	return rule
}

func TestRuleStructuralIdentityAndSourceHash(t *testing.T) {
	f := newSubsumptionFixture(t)
	first := f.rule("first-name", f.p, []ast.ASTHandle{f.q, f.r}, []bool{false, true})
	second := f.rule("different-name", f.p, []ast.ASTHandle{f.q, f.r}, []bool{false, true})
	polarity := f.rule("polarity", f.p, []ast.ASTHandle{f.q, f.r}, []bool{false, false})
	differentHead := f.rule("head", f.q, []ast.ASTHandle{f.q, f.r}, []bool{false, true})
	defer func() {
		ruleMust(t, differentHead.Close())
		ruleMust(t, polarity.Close())
		ruleMust(t, second.Close())
		ruleMust(t, first.Close())
	}()

	equal, err := first.Equal(second)
	ruleMust(t, err)
	if !equal {
		t.Fatal("names participated in structural rule equality")
	}
	firstHash, err := first.Hash()
	ruleMust(t, err)
	secondHash, err := second.Hash()
	ruleMust(t, err)
	if firstHash != secondHash {
		t.Fatal("equal rules had different source hashes", firstHash, secondHash)
	}
	for _, other := range []*base.Rule{polarity, differentHead} {
		equal, err = first.Equal(other)
		ruleMust(t, err)
		if equal {
			t.Fatal("structurally different rule compared equal")
		}
	}
	if same, err := first.Same(second); err != nil || same {
		t.Fatal("structural equality collapsed owner identity", same, err)
	}
}

func TestRuleSubsumptionIndexFactsExactRulesResetAndOwnership(t *testing.T) {
	f := newSubsumptionFixture(t)
	fact := f.rule("fact", f.p, nil, nil)
	conditional := f.rule("conditional", f.q, []ast.ASTHandle{f.r}, nil)
	duplicate := f.rule("duplicate", f.q, []ast.ASTHandle{f.r}, nil)
	changed := f.rule("changed", f.q, []ast.ASTHandle{f.p}, nil)
	defer func() {
		ruleMust(t, changed.Close())
		ruleMust(t, duplicate.Close())
		ruleMust(t, conditional.Close())
		ruleMust(t, fact.Close())
	}()
	index, err := base.NewRuleSubsumptionIndex(f.rules)
	ruleMust(t, err)
	defer func() { ruleMust(t, index.Close()) }()
	ruleMust(t, index.Add(fact))
	ruleMust(t, index.Add(conditional))

	for _, test := range []struct {
		rule *base.Rule
		want bool
	}{{fact, true}, {duplicate, true}, {changed, false}} {
		got, err := index.IsSubsumedRule(test.rule)
		ruleMust(t, err)
		if got != test.want {
			t.Fatal("rule subsumption", got, "want", test.want)
		}
	}
	if got, err := index.IsSubsumedApplication(f.p); err != nil || !got {
		t.Fatal("unconditional head was not indexed", got, err)
	}
	if got, err := index.IsSubsumedApplication(f.q); err != nil || got {
		t.Fatal("conditional head indexed as fact", got, err)
	}
	copyOwner := *index
	if _, err = copyOwner.IsSubsumedRule(fact); !errors.Is(err, base.ErrRuleSubsumptionIndex) {
		t.Fatal("copied index accepted", err)
	}
	ruleMust(t, index.Reset())
	if got, err := index.IsSubsumedRule(fact); err != nil || got {
		t.Fatal("reset retained fact", got, err)
	}
}

func TestBasicRuleSubsumptionAndWeakerRuleSets(t *testing.T) {
	f := newSubsumptionFixture(t)
	short := f.rule("short", f.p, []ast.ASTHandle{f.q}, []bool{false})
	long := f.rule("long", f.p, []ast.ASTHandle{f.q, f.r}, []bool{true, false})
	otherHead := f.rule("other", f.q, []ast.ASTHandle{f.q, f.r}, nil)
	defer func() {
		ruleMust(t, otherHead.Close())
		ruleMust(t, long.Close())
		ruleMust(t, short.Close())
	}()
	if got, err := base.BasicRuleSubsumes(short, long); err != nil || !got {
		t.Fatal("short source rule did not subsume stronger body", got, err)
	}
	if got, err := base.BasicRuleSubsumes(long, short); err != nil || got {
		t.Fatal("subsumption was not asymmetric", got, err)
	}
	if got, err := base.BasicRuleSubsumes(short, otherHead); err != nil || got {
		t.Fatal("different head was subsumed", got, err)
	}

	oldSet, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	newSet, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	empty, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	defer func() {
		ruleMust(t, empty.Close())
		ruleMust(t, newSet.Close())
		ruleMust(t, oldSet.Close())
	}()
	ruleMust(t, oldSet.Add(short))
	ruleMust(t, newSet.Add(long))
	if got, err := base.RuleSetIsWeaker(oldSet, newSet); err != nil || !got {
		t.Fatal("weaker-rule-set reuse condition", got, err)
	}
	if got, err := base.RuleSetIsWeaker(newSet, oldSet); err != nil || got {
		t.Fatal("stronger rule set was classified weaker", got, err)
	}
	if got, err := base.RuleSetIsWeaker(empty, newSet); err != nil || got {
		t.Fatal("empty old snapshot was reusable", got, err)
	}
}

func TestRuleSetSnapshotCheckReplaceResetAndOwnership(t *testing.T) {
	f := newSubsumptionFixture(t)
	short := f.rule("short", f.p, []ast.ASTHandle{f.q}, nil)
	long := f.rule("long", f.p, []ast.ASTHandle{f.q, f.r}, nil)
	different := f.rule("different", f.q, []ast.ASTHandle{f.r}, nil)
	defer func() {
		ruleMust(t, different.Close())
		ruleMust(t, long.Close())
		ruleMust(t, short.Close())
	}()
	makeSet := func(rules ...*base.Rule) *base.RuleSet {
		t.Helper()
		set, err := base.NewRuleSet(f.rules)
		ruleMust(t, err)
		for _, rule := range rules {
			ruleMust(t, set.Add(rule))
		}
		return set
	}
	first := makeSet(short)
	weaker := makeSet(long)
	unrelated := makeSet(different)
	defer func() {
		ruleMust(t, unrelated.Close())
		ruleMust(t, weaker.Close())
		ruleMust(t, first.Close())
	}()
	snapshot, err := base.NewRuleSetSnapshot(f.rules)
	ruleMust(t, err)
	defer func() { ruleMust(t, snapshot.Close()) }()
	if reuse, err := snapshot.CheckAndReplace(first); err != nil || reuse {
		t.Fatal("empty initial snapshot was reusable", reuse, err)
	}
	if reuse, err := snapshot.CheckAndReplace(weaker); err != nil || !reuse {
		t.Fatal("weaker replacement did not reuse snapshot", reuse, err)
	}
	if reuse, err := snapshot.CheckAndReplace(unrelated); err != nil || reuse {
		t.Fatal("unrelated replacement reused snapshot", reuse, err)
	}
	copyOwner := *snapshot
	if _, err = copyOwner.CheckAndReplace(first); !errors.Is(err, base.ErrRuleSetSnapshot) {
		t.Fatal("copied snapshot accepted", err)
	}
	ruleMust(t, snapshot.Reset())
	if reuse, err := snapshot.CheckAndReplace(first); err != nil || reuse {
		t.Fatal("reset snapshot retained old rules", reuse, err)
	}

	foreignRules, err := base.NewRuleManager(f.manager)
	ruleMust(t, err)
	foreign, err := base.NewRuleSet(foreignRules)
	ruleMust(t, err)
	if _, err = snapshot.CheckAndReplace(foreign); !errors.Is(err, base.ErrRuleSetSnapshot) {
		t.Fatal("foreign rule set accepted", err)
	}
	ruleMust(t, foreign.Close())
	ruleMust(t, foreignRules.Close())
}
