package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogContextRuleScopesAndVariableBinding(t *testing.T) {
	f := newRuleSetFixture(t)
	c, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, c.Close()) })
	b, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("ctx-p")
	ruleMust(t, err)
	p, err := f.manager.MkFuncDecl(name, nil, b, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	pa, err := f.manager.MkApp(p, nil)
	ruleMust(t, err)
	ruleMust(t, c.RegisterPredicate(p, true))
	ruleMust(t, c.SetOutputPredicate(p))
	rm, err := c.RuleManager()
	ruleMust(t, err)
	r1, err := rm.NewRule(pa, nil, nil, "fact-one")
	ruleMust(t, err)
	defer func() { ruleMust(t, r1.Close()) }()
	ruleMust(t, c.AddRule(r1))
	ruleMust(t, c.Push())
	r2, err := rm.NewRule(pa, nil, nil, "fact-two")
	ruleMust(t, err)
	defer func() { ruleMust(t, r2.Close()) }()
	ruleMust(t, c.AddRule(r2))
	if n, e := c.RuleCount(); e != nil || n != 2 {
		t.Fatalf("scoped count = %d,%v", n, e)
	}
	if _, e := c.Finalize(); e != nil {
		t.Fatal(e)
	}
	ruleMust(t, c.Pop(1))
	if n, e := c.RuleCount(); e != nil || n != 1 {
		t.Fatalf("restored count = %d,%v", n, e)
	}

	xname, err := f.manager.Symbols.Intern("ctx-x")
	ruleMust(t, err)
	x, err := f.manager.MkFuncDecl(xname, nil, b, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, c.RegisterVariable(x))
	xa, err := f.manager.MkApp(x, nil)
	ruleMust(t, err)
	bound, err := c.BindVariables(xa, true)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&bound)) }()
	boundHandle, err := f.manager.RefGet(bound)
	ruleMust(t, err)
	if kind, e := f.manager.GetKind(boundHandle); e != nil || kind != ast.ASTQuantifier {
		t.Fatalf("bound kind = %v,%v", kind, e)
	}
}
