//go:build g3trace

package base

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type scopeCallbackWriter struct {
	callback func()
	calls    int
}

func (w *scopeCallbackWriter) Write(p []byte) (int, error) {
	if w.callback != nil {
		w.calls++
		w.callback()
	}
	return len(p), nil
}

// The trace writer is invoked synchronously by actual AST destruction while
// Pop removes a scoped background assertion. It attempts operations through a
// previously borrowed RuleSet, as well as reentering the context itself.
func TestDatalogScopeRecoveryReentrantTraceCallback(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	rt, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: rt, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	writer := &scopeCallbackWriter{}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget, TraceTags: tags})
	must(err)
	context, err := NewDatalogContext(manager)
	must(err)
	var padding []ast.ASTRef
	defer func() {
		writer.callback = nil
		budget.FailAfter(-1)
		for i := range padding {
			must(manager.DelRef(&padding[i]))
		}
		must(context.Close())
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(rt.Close())
		if budget.Used() != 0 {
			t.Fatal("scope callback fixture leaked bytes", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	head, err := manager.MkFreshConst("scope-callback-saved", boolean)
	must(err)
	decl, err := manager.GetDecl(head)
	must(err)
	must(context.RegisterPredicate(decl, false))
	rm, err := context.RuleManager()
	must(err)
	saved, err := rm.NewRule(head, nil, nil, "saved")
	must(err)
	defer func() { must(saved.Close()) }()
	must(context.AddRule(saved))
	must(context.SetOutputPredicate(decl))
	rules, err := context.Rules()
	must(err)
	must(rules.InheritPredicate(rules, decl, decl))
	must(context.Push())
	must(rules.Reset())
	current, err := rm.NewRule(head, nil, nil, "current")
	must(err)
	defer func() { must(current.Close()) }()
	must(rules.Add(current))
	assertion, err := manager.MkFreshConst("scope-callback-assertion", boolean)
	must(err)
	must(context.AssertExpression(assertion))
	budget.FailAfter(0)
	for i := 0; ; i++ {
		ref, cause := manager.NewRef(ast.ASTHandle{})
		if errors.Is(cause, arena.ErrInjectedFailure) {
			break
		}
		must(cause)
		padding = append(padding, ref)
		if i >= 256 {
			t.Fatal("reference page exhaustion not reached")
		}
	}
	intact := true
	tags.Enable("ast")
	writer.callback = func() {
		_, e := rules.Len()
		intact = intact && errors.Is(e, ErrRuleSetOwner)
		intact = intact && errors.Is(rules.Reset(), ErrRuleSetOwner)
		intact = intact && errors.Is(rules.ReplaceRules(rules), ErrRuleSetOwner)
		intact = intact && errors.Is(rules.Close(), ErrRuleSetOwner)
		intact = intact && errors.Is(context.Pop(1), ErrDatalogContextOwner)
		intact = intact && errors.Is(context.Close(), ErrDatalogContextOwner)
		_, e = context.Rules()
		intact = intact && errors.Is(e, ErrDatalogContextOwner)
	}
	err = context.Pop(1)
	writer.callback = nil
	tags.Disable("ast")
	if !errors.Is(err, arena.ErrInjectedFailure) || writer.calls == 0 || !intact {
		t.Fatal("scope recovery callback or allocation refusal", err, writer.calls, intact)
	}
	if len(context.scopes) != 1 || !context.scopes[0].popping || context.scopes[0].restored || len(context.background) != 0 || len(rules.rules) != 1 || rules.rules[0].state != current.state {
		t.Fatal("failed pop lost ownership or advanced the snapshot")
	}
	budget.FailAfter(-1)
	must(context.Pop(1))
	got, err := rules.Rule(0)
	must(err)
	same, err := got.Same(saved)
	must(err)
	must(got.Close())
	output, err := rules.IsOutputPredicate(decl)
	must(err)
	if !same || !output || len(context.scopes) != 0 {
		t.Fatal("retry lost saved rule or output", same, output)
	}
}
