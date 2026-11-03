package base_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// Pending context normalization must publish new contents without replacing
// the RuleSet object borrowed by the context's scope stack.
func TestRuleSetExchangeContentsTransfersMetadataAndOwners(t *testing.T) {
	f := newRuleSetFixture(t, "old", "input", "new", "original-old", "original-new")
	newSet := func() *base.RuleSet {
		s, err := base.NewRuleSet(f.rules)
		ruleMust(t, err)
		t.Cleanup(func() { ruleMust(t, s.Close()) })
		return s
	}
	originals, live, replacement := newSet(), newSet(), newSet()
	ruleMust(t, originals.SetOutputPredicate(f.decl["original-old"]))
	ruleMust(t, originals.SetOutputPredicate(f.decl["original-new"]))
	ruleMust(t, live.InheritPredicate(originals, f.decl["original-old"], f.decl["old"]))
	ruleMust(t, replacement.InheritPredicate(originals, f.decl["original-new"], f.decl["new"]))
	old := f.rule("old", nil, nil)
	ruleMust(t, live.Add(old))
	ruleMust(t, old.Close())
	for _, rule := range []*base.Rule{
		f.rule("input", nil, nil),
		f.rule("new", []string{"input"}, []bool{true}),
	} {
		ruleMust(t, replacement.Add(rule))
		ruleMust(t, rule.Close())
	}
	finalized, err := replacement.Finalize()
	ruleMust(t, err)
	if !finalized {
		t.Fatal("replacement program was not stratified")
	}
	oldHeld, err := live.Rule(0)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, oldHeld.Close()) })
	newHeld, err := replacement.Rule(1)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, newHeld.Close()) })
	borrowed := live
	before := f.manager.Stats()
	ruleMust(t, live.ExchangeContents(replacement))
	after := f.manager.Stats()
	if before.LiveNodes != after.LiveNodes || before.LiveReferences != after.LiveReferences {
		t.Fatal("content exchange changed AST ownership", before, after)
	}
	if count, err := borrowed.Len(); err != nil || count != 2 {
		t.Fatal("borrowed live object did not see replacement rules", count, err)
	}
	if count, err := replacement.Len(); err != nil || count != 1 {
		t.Fatal("replacement owner did not receive old rules", count, err)
	}
	for _, tc := range []struct {
		set                 *base.RuleSet
		original, predicate string
		finalized           bool
	}{{live, "original-new", "new", true}, {replacement, "original-old", "old", false}} {
		outputs, err := tc.set.OutputPredicates()
		ruleMust(t, err)
		if !reflect.DeepEqual(outputs, []ast.ASTHandle{f.decl[tc.predicate]}) {
			t.Fatal("output ownership did not move", outputs)
		}
		original, err := tc.set.OriginalPredicate(f.decl[tc.predicate])
		ruleMust(t, err)
		derived, err := tc.set.DerivedPredicate(f.decl[tc.original])
		ruleMust(t, err)
		if original != f.decl[tc.original] || derived != f.decl[tc.predicate] {
			t.Fatal("predicate mapping did not move", original, derived)
		}
		got, err := tc.set.IsFinalized()
		ruleMust(t, err)
		if got != tc.finalized {
			t.Fatal("finalization state did not move", got, tc.finalized)
		}
	}
	dependencies, err := live.Dependencies(f.decl["new"])
	ruleMust(t, err)
	if !reflect.DeepEqual(dependencies, []ast.ASTHandle{f.decl["input"]}) {
		t.Fatal("replacement dependencies were lost", dependencies)
	}
	strata, err := live.Strata()
	ruleMust(t, err)
	if !reflect.DeepEqual(strata, [][]ast.ASTHandle{{f.decl["input"]}, {f.decl["new"]}}) {
		t.Fatal("replacement strata or head ordering were lost", strata)
	}
	group, err := live.PredicateRules(f.decl["new"])
	ruleMust(t, err)
	if len(group) != 1 {
		t.Fatal("replacement head index was lost", len(group))
	}
	same, err := group[0].Same(newHeld)
	ruleMust(t, err)
	ruleMust(t, group[0].Close())
	if !same {
		t.Fatal("head index changed the underlying rule")
	}
	ruleMust(t, replacement.Close())
	if head, err := oldHeld.Head(); err != nil || head != f.app["old"] {
		t.Fatal("closing the old set destroyed an independent rule owner", head, err)
	}
	ruleMust(t, live.Close())
	if head, err := newHeld.Head(); err != nil || head != f.app["new"] {
		t.Fatal("closing the live set destroyed an independent rule owner", head, err)
	}
}

func TestRuleSetExchangeContentsRejectsInvalidOwnersWithoutMutation(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	live, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, live.Close()) })
	rule := f.rule("P", nil, nil)
	ruleMust(t, live.Add(rule))
	ruleMust(t, rule.Close())
	otherManager, err := base.NewRuleManager(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, otherManager.Close()) })
	foreign, err := base.NewRuleSet(otherManager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, foreign.Close()) })
	closed, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	ruleMust(t, closed.Close())
	var absent *base.RuleSet
	for _, pair := range [][2]*base.RuleSet{{live, absent}, {absent, live}, {live, foreign}, {foreign, live}, {live, closed}, {closed, live}} {
		if err := pair[0].ExchangeContents(pair[1]); !errors.Is(err, base.ErrRuleSetOwner) {
			t.Fatal("invalid exchange owner accepted", err)
		}
		if count, err := live.Len(); err != nil || count != 1 {
			t.Fatal("rejected exchange changed the live program", count, err)
		}
	}
	ruleMust(t, live.ExchangeContents(live))
	if count, err := live.Len(); err != nil || count != 1 {
		t.Fatal("self exchange changed the live program", count, err)
	}
}
