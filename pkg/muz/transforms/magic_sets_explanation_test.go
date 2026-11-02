package transforms_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"testing"
)

func TestMagicSetsTransfersExistingExplanationRelation(t *testing.T) {
	t.Run("populated", func(t *testing.T) { testMagicSetsTransfersExistingExplanationRelation(t, true) })
	t.Run("empty", func(t *testing.T) { testMagicSetsTransfersExistingExplanationRelation(t, false) })
}

func testMagicSetsTransfersExistingExplanationRelation(t *testing.T, populated bool) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	ruleSort, err := f.manager.MkDatalogRuleSort()
	coneMust(t, err)
	termVariable, err := f.manager.MkVar(1, ruleSort)
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	declare := func(name string) (ast.ASTHandle, ast.ASTHandle) {
		symbol, e := f.manager.Symbols.Intern("magic-transfer-" + name)
		coneMust(t, e)
		declaration, e := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{boolean, ruleSort}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, e)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		application, e := f.manager.MkApp(declaration, []ast.ASTHandle{x, termVariable})
		coneMust(t, e)
		return declaration, application
	}
	_, extensional := declare("E")
	qDecl, q := declare("Q")
	_, predicate := declare("P")
	goalDecl, goal := declare("G")
	addTransformRule(t, f, q, nil, nil, "q-rule")
	addTransformRule(t, f, predicate, []ast.ASTHandle{extensional, q}, nil, "p-rule")
	addTransformRule(t, f, goal, []ast.ASTHandle{predicate}, nil, "goal-rule")
	coneMust(t, f.set.SetOutputPredicate(goalDecl))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("magic transfer fixture is unstratified")
	}
	relations, err := rel.NewRelationManager(f.manager)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, relations.Close()) })
	product, err := relations.GetExplanationRelation(qDecl)
	coneMust(t, err)
	if populated {
		name, err := f.manager.Symbols.Intern("magic-stored-proof")
		coneMust(t, err)
		term, err := f.manager.MkDatalogRule(name, nil)
		coneMust(t, err)
		truth, err := f.manager.MkTrue()
		coneMust(t, err)
		coneMust(t, product.AddFact([]ast.ASTHandle{truth}, term))
	}
	plugin := transforms.NewMagicSetsPluginWithRelations(f.manager, f.rules, goalDecl, true, 0, relations)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("enabled magic sets returned no transformation")
	}
	defer func() { coneMust(t, result.Close()) }()
	transfers := 0
	count, err := result.Len()
	coneMust(t, err)
	for i := 0; i < count; i++ {
		rule, getErr := result.Rule(i)
		coneMust(t, getErr)
		defer func() { coneMust(t, rule.Close()) }()
		count, getErr := rule.TailSize()
		coneMust(t, getErr)
		if count != 2 {
			continue
		}
		first, _, getErr := rule.Tail(0)
		coneMust(t, getErr)
		declaration, getErr := f.manager.GetDecl(first)
		coneMust(t, getErr)
		if declaration == qDecl {
			transfers++
		}
	}
	want := 0
	if populated {
		want = 1
	}
	if transfers != want {
		t.Fatalf("existing Q relation produced %d adorned transfer rules, want %d", transfers, want)
	}
}
