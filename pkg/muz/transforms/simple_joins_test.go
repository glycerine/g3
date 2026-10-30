package transforms_test

// Source: src/muz/rel/dl_mk_simple_joins.cpp planner-generated predicate names.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestSimpleJoinsUsesSourceSplitPredicateNames(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, context.Close()) })
	manager, err := context.Manager()
	coneMust(t, err)
	rules, err := base.NewRuleManager(manager)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, rules.Close()) })
	set, err := base.NewRuleSet(rules)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, set.Close()) })
	boolean, err := manager.MkBoolSort()
	coneMust(t, err)
	applications := make([]ast.ASTHandle, 4)
	for index, name := range []string{"A", "B", "C", "Goal"} {
		declarationName, x := manager.Symbols.Intern("simple-name-" + name)
		coneMust(t, x)
		declaration, x := manager.MkFuncDecl(declarationName, nil, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, x)
		coneMust(t, rules.RegisterPredicate(declaration))
		applications[index], x = manager.MkApp(declaration, nil)
		coneMust(t, x)
	}
	rule, err := rules.NewRule(applications[3], applications[:3], nil, "three-tail")
	coneMust(t, err)
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())
	plugin := transforms.NewSimpleJoinsPlugin(manager, rules, 0)
	transformed, err := plugin.Transform(set)
	coneMust(t, err)
	if transformed == nil {
		t.Fatal("three-tail rule was not transformed")
	}
	t.Cleanup(func() {
		coneMust(t, transformed.Close())
		coneMust(t, plugin.Close())
	})
	fresh := plugin.FreshPredicates()
	if len(fresh) != 1 {
		t.Fatalf("fresh split predicates = %d, want 1", len(fresh))
	}
	name, err := manager.GetASTName(fresh[0])
	coneMust(t, err)
	text, err := manager.Symbols.String(name)
	coneMust(t, err)
	if !strings.HasPrefix(text, "simple-name-Goal!split!") {
		t.Fatalf("split predicate name %q does not retain the source parent and split suffix", text)
	}
}
