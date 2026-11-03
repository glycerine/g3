package transforms_test

// Source: context::mk_fresh_head_predicate inherits relation kind from the
// direct orig_pred supplied by MagicSets for adorned and magic declarations.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestMagicSetsRecordsDirectFreshPredicateOrigins(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	declare := func(name string) (ast.ASTHandle, ast.ASTHandle) {
		t.Helper()
		symbol, declarationErr := f.manager.Symbols.Intern("magic-origin-" + name)
		coneMust(t, declarationErr)
		declaration, declarationErr := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, declarationErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		application, declarationErr := f.manager.MkApp(declaration, []ast.ASTHandle{x})
		coneMust(t, declarationErr)
		return declaration, application
	}
	_, extensional := declare("E")
	_, predicate := declare("P")
	goalDecl, goal := declare("Goal")
	addTransformRule(t, f, predicate, []ast.ASTHandle{extensional}, nil, "magic-origin-p")
	addTransformRule(t, f, goal, []ast.ASTHandle{predicate}, nil, "magic-origin-goal")
	coneMust(t, f.set.SetOutputPredicate(goalDecl))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("magic origin fixture rejected")
	}

	plugin := transforms.NewMagicSetsPlugin(f.manager, f.rules, goalDecl, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("magic origin transform returned nil")
	}
	defer func() { coneMust(t, result.Close()) }()
	fresh := plugin.FreshPredicates()
	if len(fresh) == 0 {
		t.Fatal("magic origin transform published no fresh predicates")
	}
	freshSet := make(map[ast.ASTHandle]bool, len(fresh))
	for _, declaration := range fresh {
		freshSet[declaration] = true
	}
	chained := false
	for _, declaration := range fresh {
		origin, found := plugin.FreshPredicateOrigin(declaration)
		if !found || origin.IsNull() {
			t.Fatal("fresh predicate missing direct origin", declaration)
		}
		chained = chained || freshSet[origin]
	}
	if !chained {
		t.Fatal("magic predicates did not point to their adorned origins")
	}
	if _, found := plugin.FreshPredicateOrigin(goalDecl); found {
		t.Fatal("original predicate reported as fresh")
	}
}
