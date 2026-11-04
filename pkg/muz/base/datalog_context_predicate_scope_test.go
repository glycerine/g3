package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// dl_context.cpp keeps the name registry across rel_context scoped-query
// predicate restoration, and only names a predicate at its first registration.
func TestDatalogContextPredicateScopeKeepsNameRegistry(t *testing.T) {
	f := newRuleSetFixture(t)
	c, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, c.Close()) })
	b, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("scope-original")
	ruleMust(t, err)
	original, err := f.manager.MkFuncDecl(name, nil, b, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, c.RegisterPredicate(original, true))
	otherName, err := f.manager.Symbols.Intern("scope-unnamed")
	ruleMust(t, err)
	other, err := f.manager.MkFuncDecl(otherName, nil, b, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, c.RegisterPredicate(other, false))
	ruleMust(t, c.RegisterPredicate(other, true))
	if _, found, err := c.Predicate(otherName); err != nil || found {
		t.Fatalf("re-registration named an existing predicate: found=%v err=%v", found, err)
	}
	ruleMust(t, c.RestrictPredicates(map[ast.ASTHandle]bool{other: true}))
	if got, found, err := c.Predicate(name); err != nil || !found || got != original {
		t.Fatalf("name after restriction = %v, %v, %v", got, found, err)
	}
	if found, err := c.IsPredicate(original); err != nil || found {
		t.Fatalf("membership after restriction = %v, %v", found, err)
	}
	ruleMust(t, c.RestrictPredicates(map[ast.ASTHandle]bool{original: true}))
	if found, err := c.IsPredicate(original); err != nil || !found {
		t.Fatalf("restored membership = %v, %v", found, err)
	}
	if found, err := c.IsPredicate(other); err != nil || found {
		t.Fatalf("removed temporary membership = %v, %v", found, err)
	}
}
