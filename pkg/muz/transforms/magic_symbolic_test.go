package transforms_test

// Source: src/muz/transforms/dl_mk_magic_symbolic.cpp answer rules, output
// query facts, and left-to-right query propagation.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestMagicSymbolicBuildsAnswerAndPrefixQueryRules(t *testing.T) {
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
	makePredicate := func(name string) (ast.ASTHandle, ast.ASTHandle) {
		symbol, makeErr := manager.Symbols.Intern(name)
		coneMust(t, makeErr)
		declaration, makeErr := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, rules.RegisterPredicate(declaration))
		application, makeErr := manager.MkApp(declaration, nil)
		coneMust(t, makeErr)
		return declaration, application
	}
	qDecl, q := makePredicate("magic-q")
	_, a := makePredicate("magic-a")
	_, b := makePredicate("magic-b")
	truth, err := manager.MkTrue()
	coneMust(t, err)
	rule, err := rules.NewRule(q, []ast.ASTHandle{a, b, truth}, nil, "magic")
	coneMust(t, err)
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, set.SetOutputPredicate(qDecl))
	valid, err := set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("magic source is unstratified")
	}
	plugin := transforms.NewMagicSymbolicPlugin(manager, rules, 36020)
	transformer := base.NewRuleTransformer()
	coneMust(t, transformer.Register(plugin))
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	modified, err := transformer.Apply(set)
	coneMust(t, err)
	if !modified {
		t.Fatal("magic transformation did not modify the rule set")
	}
	answer := plugin.AnswerPredicates()[qDecl]
	if answer.IsNull() {
		t.Fatal("missing output answer predicate")
	}
	if original, getErr := set.OriginalPredicate(answer); getErr != nil || original != qDecl {
		t.Fatal("answer predicate mapping", original, getErr)
	}
	if count, getErr := set.Len(); getErr != nil || count != 4 {
		t.Fatal("magic rule count", count, getErr)
	}
	wantTails := map[int]int{0: 4, 1: 0, 2: 2, 3: 3}
	for i := 0; i < 4; i++ {
		transformed, getErr := set.Rule(i)
		coneMust(t, getErr)
		tails, getErr := transformed.TailSize()
		coneMust(t, getErr)
		coneMust(t, transformed.Close())
		if tails != wantTails[i] {
			t.Fatal("magic prefix tail count", i, tails, wantTails[i])
		}
	}
}

func TestMagicSymbolicReusesTypedDerivedPredicates(t *testing.T) {
	f := newConeFixture(t)
	integer, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	p := declareTransformPredicate(t, f, "magic-typed-p", []ast.ASTHandle{integer}, []ast.ASTHandle{x})
	q := declareTransformPredicate(t, f, "magic-typed-q", []ast.ASTHandle{integer}, []ast.ASTHandle{x})
	addTransformRule(t, f, q.application, []ast.ASTHandle{p.application}, nil, "first")
	addTransformRule(t, f, q.application, []ast.ASTHandle{p.application}, nil, "second")
	coneMust(t, f.set.SetOutputPredicate(q.declaration))

	plugin := transforms.NewMagicSymbolicPlugin(f.manager, f.rules, 36020)
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	if result == nil {
		t.Fatal("typed magic transformation returned nil")
	}
	t.Cleanup(func() { coneMust(t, result.Close()) })
	answer := plugin.AnswerPredicates()[q.declaration]
	if answer.IsNull() {
		t.Fatal("missing typed answer predicate")
	}
	if domain, getErr := f.manager.GetDomain(answer, 0); getErr != nil || domain != integer {
		t.Fatal("typed answer domain", domain, getErr)
	}
	if count, getErr := result.Len(); getErr != nil || count != 6 {
		t.Fatal("two source rules should each produce answer, seed, and prefix rules", count, getErr)
	}
	declarations := make(map[string]map[ast.ASTHandle]struct{})
	for i := 0; i < 6; i++ {
		rule, getErr := result.Rule(i)
		coneMust(t, getErr)
		head, getErr := rule.Head()
		coneMust(t, getErr)
		decl, getErr := f.manager.GetDecl(head)
		coneMust(t, getErr)
		name, getErr := f.manager.GetASTName(decl)
		coneMust(t, getErr)
		text, getErr := f.manager.Symbols.String(name)
		coneMust(t, getErr)
		if declarations[text] == nil {
			declarations[text] = make(map[ast.ASTHandle]struct{})
		}
		declarations[text][decl] = struct{}{}
		coneMust(t, rule.Close())
	}
	for _, name := range []string{"magic-typed-q!ans", "magic-typed-q!query", "magic-typed-p!query"} {
		if len(declarations[name]) != 1 {
			t.Fatal("derived predicate declaration was not cached", name, declarations[name])
		}
	}
}
