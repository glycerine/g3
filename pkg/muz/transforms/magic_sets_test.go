package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestMagicSetsAdornsIntentionalChainAndSeedsGoal(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	declare := func(name string) (ast.ASTHandle, ast.ASTHandle) {
		symbol, e := f.manager.Symbols.Intern(name)
		coneMust(t, e)
		declaration, e := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, e)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		application, e := f.manager.MkApp(declaration, []ast.ASTHandle{x})
		coneMust(t, e)
		return declaration, application
	}
	extensionalDecl, extensional := declare("magic-E")
	_, _ = extensionalDecl, extensional
	qDecl, q := declare("magic-Q")
	pDecl, p := declare("magic-P")
	goalDecl, goal := declare("magic-Goal")
	addTransformRule(t, f, q, nil, nil, "q-fact")
	addTransformRule(t, f, p, []ast.ASTHandle{extensional, q}, nil, "p-rule")
	addTransformRule(t, f, goal, []ast.ASTHandle{p}, nil, "goal-rule")
	coneMust(t, f.set.SetOutputPredicate(goalDecl))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("magic fixture rejected")
	}
	plugin := transforms.NewMagicSetsPlugin(f.manager, f.rules, goalDecl, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("enabled magic sets returned no transformation")
	}
	defer func() { coneMust(t, result.Close()) }()
	if len(plugin.FreshPredicates()) < 4 {
		t.Fatal("missing adorned or magic predicates", len(plugin.FreshPredicates()))
	}
	goalRules, err := result.PredicateRules(goalDecl)
	coneMust(t, err)
	if len(goalRules) != 1 {
		t.Fatalf("back-to-goal rules %d, want 1", len(goalRules))
	}
	if tails, err := goalRules[0].TailSize(); err != nil || tails != 1 {
		t.Fatal("back-to-goal tail", tails, err)
	}
	coneMust(t, goalRules[0].Close())
	magicFacts := 0
	count, err := result.Len()
	coneMust(t, err)
	for i := 0; i < count; i++ {
		rule, e := result.Rule(i)
		coneMust(t, e)
		tails, e := rule.TailSize()
		if e == nil && tails == 0 {
			head, x := rule.HeadDeclaration()
			if x == nil {
				arity, x := f.manager.GetArity(head)
				if x == nil && arity == 0 && head != qDecl && head != pDecl {
					magicFacts++
				}
			}
			e = x
		}
		coneMust(t, e)
		coneMust(t, rule.Close())
	}
	if magicFacts == 0 {
		t.Fatal("goal magic seed fact missing")
	}
}

func TestMagicSetsDisabledAndGoalMultiplicityGuards(t *testing.T) {
	f := newConeFixture(t, "Goal")
	f.add(t, f.applications["Goal"], "goal-1")
	f.add(t, f.applications["Goal"], "goal-2")
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("guard fixture rejected")
	}
	disabled := transforms.NewMagicSetsPlugin(f.manager, f.rules, f.declarations["Goal"], false, 0)
	result, err := disabled.Transform(f.set)
	coneMust(t, err)
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("disabled magic sets changed rules")
	}
	coneMust(t, disabled.Close())
	enabled := transforms.NewMagicSetsPlugin(f.manager, f.rules, f.declarations["Goal"], true, 0)
	if result, err = enabled.Transform(f.set); err == nil {
		if result != nil {
			coneMust(t, result.Close())
		}
		t.Fatal("multiple goal rules accepted")
	}
	coneMust(t, enabled.Close())
}
