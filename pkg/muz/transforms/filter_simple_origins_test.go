package transforms_test

// Source: dl_mk_filter_rules.cpp:85-86 and dl_mk_simple_joins.cpp:385-399
// pass the direct source predicate to context::mk_fresh_head_predicate.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestSimpleJoinsOriginUsesLastRegisteredSharedConsumer(t *testing.T) {
	f := newConeFixture(t, "A", "B", "C", "H1", "H2", "Short")
	tails := []ast.ASTHandle{f.applications["A"], f.applications["B"], f.applications["C"]}
	for _, name := range []string{"H1", "H2"} {
		addTransformRule(t, f, f.applications[name], tails, nil, name)
	}
	// The last pair consumer is already short enough for the compiler. Source
	// registration still chooses its head as the shared split's kind origin.
	addTransformRule(t, f, f.applications["Short"], tails[:2], nil, "short")
	plugin := transforms.NewSimpleJoinsPlugin(f.manager, f.rules, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("shared three-tail rules were not transformed")
	}
	defer func() { coneMust(t, result.Close()) }()
	fresh := plugin.FreshPredicates()
	if len(fresh) != 1 {
		t.Fatal("shared pair should create exactly one auxiliary", len(fresh))
	}
	if origin, found := plugin.FreshPredicateOrigin(fresh[0]); !found || origin != f.declarations["Short"] {
		t.Fatal("split origin is not the last registered pair consumer", origin, found)
	}
	for i, name := range []string{"H1", "H2", "Short"} {
		rule, cause := result.Rule(i)
		coneMust(t, cause)
		declaration, cause := rule.TailDeclaration(0)
		coneMust(t, cause)
		coneMust(t, rule.Close())
		if declaration != fresh[0] {
			t.Fatal("consumer did not reuse shared auxiliary", name, declaration)
		}
	}
	if _, found := plugin.FreshPredicateOrigin(f.declarations["Short"]); found {
		t.Fatal("original predicate reported as fresh")
	}
	coneMust(t, plugin.Close())
	if _, found := plugin.FreshPredicateOrigin(fresh[0]); found {
		t.Fatal("closed join plugin published origin")
	}
	var absent *transforms.SimpleJoinsPlugin
	if _, found := absent.FreshPredicateOrigin(fresh[0]); found {
		t.Fatal("nil join plugin published origin")
	}
}

func TestFilterRulesOriginsFollowTailsAndCacheReplacement(t *testing.T) {
	for _, constant := range []bool{false, true} {
		name := "repeated"
		if constant {
			name = "constant"
		}
		t.Run(name, func(t *testing.T) {
			f := newConeFixture(t)
			boolean, err := f.manager.MkBoolSort()
			coneMust(t, err)
			source := separateDeclaration(t, f, "origin-tail", []ast.ASTHandle{boolean, boolean}, boolean)
			x, err := f.manager.MkVar(0, boolean)
			coneMust(t, err)
			second := x
			if constant {
				second, err = f.manager.MkTrue()
				coneMust(t, err)
			}
			tail, err := f.manager.MkApp(source, []ast.ASTHandle{x, second})
			coneMust(t, err)
			for _, name := range []string{"origin-first", "origin-last"} {
				declaration := separateDeclaration(t, f, name, []ast.ASTHandle{boolean}, boolean)
				head, cause := f.manager.MkApp(declaration, []ast.ASTHandle{x})
				coneMust(t, cause)
				addTransformRule(t, f, head, []ast.ASTHandle{tail}, nil, name)
			}
			plugin := transforms.NewFilterRulesPlugin(f.manager, f.rules, 0)
			defer func() { coneMust(t, plugin.Close()) }()
			var previous ast.ASTHandle
			for attempt := 0; attempt < 2; attempt++ {
				result, cause := plugin.Transform(f.set)
				coneMust(t, cause)
				if result == nil {
					t.Fatal("filter was not created")
				}
				defer func() { coneMust(t, result.Close()) }()
				fresh := plugin.FreshPredicates()
				if len(fresh) != 1 {
					t.Fatal("equivalent tails did not share filter", len(fresh))
				}
				if count, cause := result.Len(); cause != nil || count != 3 {
					t.Fatal("expected two parents and one filter definition", count, cause)
				}
				if origin, found := plugin.FreshPredicateOrigin(fresh[0]); !found || origin != source {
					t.Fatal("filter origin is not its tail declaration", origin, found)
				}
				if _, found := plugin.FreshPredicateOrigin(source); found {
					t.Fatal("source predicate reported as fresh")
				}
				if !previous.IsNull() {
					if previous == fresh[0] {
						t.Fatal("second transform did not replace filter cache")
					}
					if _, found := plugin.FreshPredicateOrigin(previous); found {
						t.Fatal("old filter origin survived cache replacement")
					}
				}
				previous = fresh[0]
			}
			coneMust(t, plugin.Close())
			if _, found := plugin.FreshPredicateOrigin(previous); found {
				t.Fatal("closed filter plugin published origin")
			}
			var absent *transforms.FilterRulesPlugin
			if _, found := absent.FreshPredicateOrigin(previous); found {
				t.Fatal("nil filter plugin published origin")
			}
		})
	}
}
