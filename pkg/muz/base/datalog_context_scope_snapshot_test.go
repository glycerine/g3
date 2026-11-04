package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogContextScopeRestoresRuleSetSnapshot(t *testing.T) {
	f := newRuleSetFixture(t)
	c, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, c.Close()) })
	b, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("scope-snapshot-p")
	ruleMust(t, err)
	p, err := f.manager.MkFuncDecl(name, nil, b, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	app, err := f.manager.MkApp(p, nil)
	ruleMust(t, err)
	ruleMust(t, c.RegisterPredicate(p, true))
	rm, err := c.RuleManager()
	ruleMust(t, err)
	one, err := rm.NewRule(app, nil, nil, "one")
	ruleMust(t, err)
	defer func() { ruleMust(t, one.Close()) }()
	two, err := rm.NewRule(app, nil, nil, "two")
	ruleMust(t, err)
	defer func() { ruleMust(t, two.Close()) }()
	ruleMust(t, c.AddRule(one))
	ruleMust(t, c.SetOutputPredicate(p))
	rules, err := c.Rules()
	ruleMust(t, err)
	ruleMust(t, c.Push())
	ruleMust(t, rules.ReplaceRule(one, two))
	qName, err := f.manager.Symbols.Intern("scope-snapshot-q")
	ruleMust(t, err)
	q, err := f.manager.MkFuncDecl(qName, nil, b, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, c.RegisterPredicate(q, true))
	ruleMust(t, c.SetOutputPredicate(q))
	ruleMust(t, c.Push())
	ruleMust(t, rules.Remove(two))
	ruleMust(t, c.Pop(1))
	got, err := rules.Rule(0)
	ruleMust(t, err)
	same, err := got.Same(two)
	ruleMust(t, err)
	ruleMust(t, got.Close())
	if !same {
		t.Fatal("inner scope did not restore replaced rule")
	}
	ruleMust(t, c.Pop(1))
	got, err = rules.Rule(0)
	ruleMust(t, err)
	same, err = got.Same(one)
	ruleMust(t, err)
	ruleMust(t, got.Close())
	if !same {
		t.Fatal("outer scope did not restore original rule")
	}
	if output, err := rules.IsOutputPredicate(p); err != nil || !output {
		t.Fatalf("restored output = %v, %v", output, err)
	}
	if output, err := rules.IsOutputPredicate(q); err != nil || output {
		t.Fatalf("scoped output retained = %v, %v", output, err)
	}
	// An outstanding snapshot is owned by Close even without Pop.
	ruleMust(t, c.Push())
	ruleMust(t, c.Close())
}

func TestDatalogContextScopeOwnsRemovedRule(t *testing.T) {
	f := newRuleSetFixture(t)
	c, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, c.Close()) })
	b, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("scope-owned-p")
	ruleMust(t, err)
	p, err := f.manager.MkFuncDecl(name, nil, b, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	app, err := f.manager.MkApp(p, nil)
	ruleMust(t, err)
	ruleMust(t, c.RegisterPredicate(p, true))
	rm, err := c.RuleManager()
	ruleMust(t, err)
	rule, err := rm.NewRule(app, nil, nil, "scope-owned")
	ruleMust(t, err)
	ruleMust(t, c.AddRule(rule))
	ruleMust(t, c.Push())
	rules, err := c.Rules()
	ruleMust(t, err)
	ruleMust(t, rules.Remove(rule))
	ruleMust(t, rule.Close())
	ruleMust(t, c.Pop(1))
	restored, err := rules.Rule(0)
	ruleMust(t, err)
	text, err := restored.Name()
	ruleMust(t, err)
	ruleMust(t, restored.Close())
	if text != "scope-owned" {
		t.Fatalf("restored rule name = %q", text)
	}
}
