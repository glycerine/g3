package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDatalogContextPredicateRegistryAndArgumentNames(t *testing.T) {
	f := newRuleSetFixture(t)
	c, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, c.Close()) })
	b, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("ctx-edge")
	ruleMust(t, err)
	d, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, c.RegisterPredicate(d, true))
	if got, ok, e := c.Predicate(name); e != nil || !ok || got != d {
		t.Fatalf("predicate lookup = %v,%v,%v", got, ok, e)
	}
	arg, err := c.ArgumentName(d, 0)
	ruleMust(t, err)
	text, err := f.manager.Symbols.Display(arg)
	ruleMust(t, err)
	if text != "#0" {
		t.Fatalf("fallback argument name = %q", text)
	}
	explicit, err := f.manager.Symbols.Intern("vertex")
	ruleMust(t, err)
	ruleMust(t, c.SetArgumentNames(d, []usymbol.Symbol{explicit}))
	arg, err = c.ArgumentName(d, 0)
	ruleMust(t, err)
	if arg != explicit {
		t.Fatal("explicit argument name not retained")
	}
	freshPrefix, err := f.manager.Symbols.Intern("ctx-fresh")
	ruleMust(t, err)
	fresh, err := c.FreshHeadPredicate(freshPrefix, usymbol.Symbol{}, []ast.ASTHandle{b})
	ruleMust(t, err)
	if ok, e := c.IsPredicate(fresh); e != nil || !ok {
		t.Fatalf("fresh predicate registration = %v,%v", ok, e)
	}
	ruleMust(t, c.RestrictPredicates(map[ast.ASTHandle]bool{fresh: true}))
	if ok, e := c.IsPredicate(d); e != nil || ok {
		t.Fatalf("restricted predicate remains = %v,%v", ok, e)
	}
}
