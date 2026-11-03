package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestRuleInlinerLinearBranchSharedAndNegativeUses(t *testing.T) {
	for _, branching := range []bool{false, true} {
		for _, negative := range []bool{false, true} {
			name := "shared"
			if negative {
				name = "negative"
			}
			if branching {
				name += "-enabled"
			}
			t.Run(name, func(t *testing.T) {
				f := newConeFixture(t, "Seed", "P", "Left", "Right")
				f.edge(t, "Seed", "P")
				f.edge(t, "P", "Left")
				rule, err := f.rules.NewRule(f.applications["Right"], []ast.ASTHandle{f.applications["P"]}, []bool{negative}, "right")
				coneMust(t, err)
				coneMust(t, f.set.Add(rule))
				coneMust(t, rule.Close())
				coneMust(t, f.set.SetOutputPredicate(f.declarations["Left"]))
				coneMust(t, f.set.SetOutputPredicate(f.declarations["Right"]))
				valid, err := f.set.Finalize()
				coneMust(t, err)
				if !valid {
					t.Fatal("branch fixture stratification")
				}
				plugin := transforms.NewRuleInlinerPluginWithModes(f.manager, f.rules, nil, false, true, 0)
				t.Cleanup(func() { coneMust(t, plugin.Close()) })
				plugin.SetLinearBranching(branching)
				coneMust(t, plugin.PreserveFacts([]ast.ASTHandle{f.declarations["Seed"]}))
				result, err := plugin.Transform(f.set)
				coneMust(t, err)
				if result != nil {
					t.Cleanup(func() { coneMust(t, result.Close()) })
				}
				if (result != nil) != branching {
					t.Fatal("branch option did not control shared expansion", result != nil, branching)
				}
				modelRules, err := plugin.TakeModelRules()
				coneMust(t, err)
				t.Cleanup(func() { coneMust(t, transforms.CloseInlineModelRules(modelRules)) })
				wantEvents := 0
				if branching && !negative {
					wantEvents = 1
				}
				if len(modelRules) != wantEvents {
					t.Fatal("definition deleted before final body use", len(modelRules), wantEvents)
				}
				if wantEvents == 1 {
					head, err := f.manager.RefGet(modelRules[0].Head)
					coneMust(t, err)
					body, err := f.manager.RefGet(modelRules[0].Body)
					coneMust(t, err)
					if head != f.applications["P"] || !f.manager.IsFalse(body) {
						t.Fatal("linear deletion did not record source false-body default")
					}
				}
				if result == nil {
					return
				}
				count, err := result.Len()
				coneMust(t, err)
				wantCount := 2
				if negative {
					wantCount = 3
				}
				if count != wantCount {
					t.Fatal("shared definition lifetime", count, wantCount)
				}
				for i := 0; i < count; i++ {
					rule, err := result.Rule(i)
					coneMust(t, err)
					head, err := rule.HeadDeclaration()
					coneMust(t, err)
					tail, negated, err := rule.Tail(0)
					coneMust(t, err)
					wantTail, wantNegative := f.applications["Seed"], false
					if negative && head == f.declarations["Right"] {
						wantTail, wantNegative = f.applications["P"], true
					}
					coneMust(t, rule.Close())
					if tail != wantTail || negated != wantNegative {
						t.Fatal("branch expansion changed shared/negative body", head, tail, negated)
					}
				}
			})
		}
	}
}

func TestRuleInlinerLinearBranchProtectsRecursiveOutput(t *testing.T) {
	f := newConeFixture(t, "P", "Q")
	f.edge(t, "Q", "P")
	f.edge(t, "P", "Q")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["P"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("recursive branch fixture")
	}
	plugin := transforms.NewRuleInlinerPluginWithModes(f.manager, f.rules, nil, false, true, 0)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	plugin.SetLinearBranching(true)
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("recursive non-output definition not contracted")
	}
	t.Cleanup(func() { coneMust(t, result.Close()) })
	count, err := result.Len()
	coneMust(t, err)
	if count != 1 {
		t.Fatal("recursive output rule count", count)
	}
	rule, err := result.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	head, err := rule.Head()
	coneMust(t, err)
	tail, negative, err := rule.Tail(0)
	coneMust(t, err)
	if head != f.applications["P"] || tail != head || negative {
		t.Fatal("recursive output protection", head, tail, negative)
	}
}

func TestRuleInlinerLinearCountsHeadsBeforeResolventSimplification(t *testing.T) {
	f := newConeFixture(t, "Seed", "P", "Goal")
	f.edge(t, "Seed", "P")
	falsity, err := f.manager.MkFalse()
	coneMust(t, err)
	unsatisfiable, err := f.rules.NewRule(f.applications["P"], []ast.ASTHandle{falsity}, nil, "false-definition")
	coneMust(t, err)
	coneMust(t, f.set.Add(unsatisfiable))
	coneMust(t, unsatisfiable.Close())
	f.edge(t, "P", "Goal")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("raw head fixture")
	}
	plugin := transforms.NewRuleInlinerPluginWithServices(f.manager, f.rules, nil, false, true, f.globals.Context, f.globals, 0)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	plugin.SetLinearBranching(true)
	coneMust(t, plugin.PreserveFacts([]ast.ASTHandle{f.declarations["Seed"]}))
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("linear selection ignored a raw head unifier after simplifying its body")
	}
}
