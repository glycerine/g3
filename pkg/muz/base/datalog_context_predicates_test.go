package base_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
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

// The source name map replaces a previous declaration with the same symbol,
// independently of predicate membership and the pinned declaration owners.
func TestDatalogContextPredicateNameReplacement(t *testing.T) {
	f := newRuleSetFixture(t)
	c, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("overloaded-context-predicate")
	ruleMust(t, err)
	first, err := f.manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	second, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	firstRef, err := f.manager.NewRef(first)
	ruleMust(t, err)
	secondRef, err := f.manager.NewRef(second)
	ruleMust(t, err)
	defer func() {
		ruleMust(t, f.manager.DelRef(&secondRef))
		ruleMust(t, f.manager.DelRef(&firstRef))
	}()
	firstCount, err := f.manager.GetRefCount(first)
	ruleMust(t, err)
	secondCount, err := f.manager.GetRefCount(second)
	ruleMust(t, err)
	checkName := func(want ast.ASTHandle) {
		t.Helper()
		got, found, err := c.Predicate(name)
		if err != nil || !found || got != want {
			t.Fatalf("lookup = %v, %v, %v; want %v", got, found, err, want)
		}
	}
	ruleMust(t, c.RegisterPredicate(first, true))
	ruleMust(t, c.RegisterPredicate(second, true))
	checkName(second)
	for _, d := range []ast.ASTHandle{first, second} {
		if found, err := c.IsPredicate(d); err != nil || !found {
			t.Fatalf("membership %v = %v, %v", d, found, err)
		}
	}
	ruleMust(t, c.RegisterPredicate(first, true))
	checkName(second)
	ruleMust(t, c.RestrictPredicates(map[ast.ASTHandle]bool{second: true}))
	checkName(second)
	ruleMust(t, c.RegisterPredicate(first, true))
	checkName(first)
	ruleMust(t, c.Close())
	for _, item := range []struct {
		d     ast.ASTHandle
		count uint32
	}{{first, firstCount}, {second, secondCount}} {
		if got, err := f.manager.GetRefCount(item.d); err != nil || got != item.count {
			t.Fatalf("reference count after close = %v, %v; want %v", got, err, item.count)
		}
	}
}

func TestDatalogContextPredicateNameReplacementAllocationRetry(t *testing.T) {
	f := newRuleSetFixture(t)
	c, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	var padding []ast.ASTRef
	defer func() {
		f.manager.BudgetField.FailAfter(-1)
		for i := range padding {
			ruleMust(t, f.manager.DelRef(&padding[i]))
		}
		ruleMust(t, c.Close())
	}()
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("overloaded-allocation-retry")
	ruleMust(t, err)
	first, err := f.manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	second, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, c.RegisterPredicate(first, true))
	f.manager.BudgetField.FailAfter(0)
	for i := 0; ; i++ {
		ref, cause := f.manager.NewRef(ast.ASTHandle{})
		if errors.Is(cause, arena.ErrInjectedFailure) {
			break
		}
		ruleMust(t, cause)
		padding = append(padding, ref)
		if i >= 256 {
			t.Fatal("reference page exhaustion not reached")
		}
	}
	if err = c.RegisterPredicate(second, true); !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatalf("registration failure = %v", err)
	}
	if got, found, err := c.Predicate(name); err != nil || !found || got != first {
		t.Fatalf("failed registration changed lookup: %v, %v, %v", got, found, err)
	}
	if found, err := c.IsPredicate(second); err != nil || found {
		t.Fatalf("failed registration changed membership: %v, %v", found, err)
	}
	f.manager.BudgetField.FailAfter(-1)
	ruleMust(t, c.RegisterPredicate(second, true))
	if got, found, err := c.Predicate(name); err != nil || !found || got != second {
		t.Fatalf("retry lookup = %v, %v, %v", got, found, err)
	}
}
