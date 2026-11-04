package base_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// The inner scope restores an empty program without allocating. The next
// restoration needs an AST reference, so exhausting the real reference
// arena makes Pop(2) fail only after it has completed one scope.
func TestDatalogContextMultiPopResumesCompletedPrefix(t *testing.T) {
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
	head, err := f.manager.MkFreshConst("multi-pop-saved", boolean)
	ruleMust(t, err)
	declaration, err := f.manager.GetDecl(head)
	ruleMust(t, err)
	ruleMust(t, c.RegisterPredicate(declaration, false))
	rm, err := c.RuleManager()
	ruleMust(t, err)
	saved, err := rm.NewRule(head, nil, nil, "saved")
	ruleMust(t, err)
	defer func() { ruleMust(t, saved.Close()) }()
	rules, err := c.Rules()
	ruleMust(t, err)
	// A sentinel scope below the request would be lost if Pop(2) were simply
	// replayed after its first scope had already completed.
	ruleMust(t, c.Push())
	ruleMust(t, c.AddRule(saved))
	ruleMust(t, c.SetOutputPredicate(declaration))
	ruleMust(t, c.Push())
	ruleMust(t, rules.Reset())
	ruleMust(t, c.Push())
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
	if err = c.Pop(2); !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected second scope allocation refusal", err)
	}
	if err = c.Pop(1); !errors.Is(err, base.ErrDatalogContextOwner) {
		t.Fatal("different retry count accepted", err)
	}
	if err = c.Pop(0); !errors.Is(err, base.ErrDatalogContextOwner) {
		t.Fatal("zero retry count accepted", err)
	}
	if _, err = rules.Len(); !errors.Is(err, base.ErrRuleSetOwner) {
		t.Fatal("partial restoration exposed borrowed rules", err)
	}
	f.manager.BudgetField.FailAfter(-1)
	ruleMust(t, c.Pop(2))
	got, err := rules.Rule(0)
	ruleMust(t, err)
	same, err := got.Same(saved)
	ruleMust(t, err)
	ruleMust(t, got.Close())
	output, err := rules.IsOutputPredicate(declaration)
	ruleMust(t, err)
	if !same || !output {
		t.Fatal("retry did not restore requested scope", same, output)
	}
	ruleMust(t, c.Pop(1))
	count, err := rules.Len()
	ruleMust(t, err)
	if count != 0 {
		t.Fatal("sentinel scope was lost or restored incorrectly", count)
	}
	if err = c.Pop(1); err == nil {
		t.Fatal("unexpected remaining scope")
	}
}
