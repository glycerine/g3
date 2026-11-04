package base_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// Exhaust real AST reference slots so staging the saved output predicate
// fails before the restored rule set is installed.
func TestDatalogContextPopRetriesSnapshotAllocationFailure(t *testing.T) {
	for _, closeInstead := range []bool{false, true} {
		name := "pop"
		if closeInstead {
			name = "close"
		}
		t.Run(name, func(t *testing.T) {
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
			head, err := f.manager.MkFreshConst("context-scope-retry", boolean)
			ruleMust(t, err)
			decl, err := f.manager.GetDecl(head)
			ruleMust(t, err)
			ruleMust(t, c.RegisterPredicate(decl, false))
			rm, err := c.RuleManager()
			ruleMust(t, err)
			saved, err := rm.NewRule(head, nil, nil, "saved")
			ruleMust(t, err)
			defer func() { ruleMust(t, saved.Close()) }()
			ruleMust(t, c.AddRule(saved))
			ruleMust(t, c.SetOutputPredicate(decl))
			rules, err := c.Rules()
			ruleMust(t, err)
			ruleMust(t, c.Push())
			ruleMust(t, rules.Reset())
			current, err := rm.NewRule(head, nil, nil, "current")
			ruleMust(t, err)
			defer func() { ruleMust(t, current.Close()) }()
			ruleMust(t, rules.Add(current))
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
			if err = c.Pop(1); !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("missing allocation refusal", err)
			}
			if err = c.Push(); !errors.Is(err, base.ErrDatalogContextOwner) {
				t.Fatal("pending pop accepted push", err)
			}
			if _, err = c.Rules(); !errors.Is(err, base.ErrDatalogContextOwner) {
				t.Fatal("pending pop exposed rules", err)
			}
			if _, err = rules.Len(); err == nil {
				t.Fatal("borrowed rules exposed a partial restore")
			}
			if err = rules.Reset(); err == nil {
				t.Fatal("borrowed rules accepted reset during pop")
			}
			if err = rules.ReplaceRules(rules); err == nil {
				t.Fatal("borrowed rules accepted replacement during pop")
			}
			if err = rules.Close(); err == nil {
				t.Fatal("borrowed rules accepted close during pop")
			}
			f.manager.BudgetField.FailAfter(-1)
			if closeInstead {
				ruleMust(t, c.Close())
				return
			}
			ruleMust(t, c.Pop(1))
			got, err := rules.Rule(0)
			ruleMust(t, err)
			same, err := got.Same(saved)
			ruleMust(t, err)
			ruleMust(t, got.Close())
			output, err := rules.IsOutputPredicate(decl)
			ruleMust(t, err)
			if !same || !output {
				t.Fatal("retry lost saved state", same, output)
			}
			if err = c.Pop(1); err == nil {
				t.Fatal("retry left an extra scope")
			}
		})
	}
}
