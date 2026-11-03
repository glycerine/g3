package bmc

// Source: dl_bmc_engine.cpp qlinear::compile nested rule/expr_ref cleanup.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifiedLinearCleanupDrainsNestedRuleReceiptBeforeIndexedRoots(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var manager *ast.ASTManager
	var q *quantifiedLinear
	var owned *base.Rule
	var results []ast.ASTRef
	var pending error
	callbacks, intact := 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		callbacks++
		if pending == nil {
			beforeQ := append([]ast.ASTRef(nil), q.refs...)
			beforeResult := results[0]
			child := disposeRuleOwners(manager, []*base.Rule{owned}, nil)
			pending = q.disposeResults(results, child)
			_, ruleErr := owned.Head()
			intact = errors.Is(pending, ast.ErrASTPending) && ruleErr == nil && results[0] == beforeResult && len(q.refs) == len(beforeQ)
			for i := range beforeQ {
				intact = intact && q.refs[i] == beforeQ[i]
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if used := budget.Used(); used != 0 {
			t.Fatal("quantified nested cleanup fixture leaked bytes", used)
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	q, err = newQuantifiedLinear(manager, 4)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(manager)
	must(err)
	defer func() { must(rules.Close()) }()
	name, err := symbols.Intern("qlinear-nested-cleanup")
	must(err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(declaration))
	head, err := manager.MkApp(declaration, nil)
	must(err)
	rule, err := rules.NewRule(head, nil, nil, "qlinear-nested-cleanup")
	must(err)
	owned, err = rule.Retain()
	must(err)
	must(rule.Close())
	result, err := manager.NewRef(head)
	must(err)
	results = []ast.ASTRef{result}

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("nested qlinear cleanup touched dependent roots", body, callbacks, intact)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(pending, &cleanup) {
		t.Fatal("nested qlinear cleanup lost retry receipt", pending)
	}
	reported, unwrapped := cleanup.Error(), cleanup.Unwrap()
	must(cleanup.Cleanup())
	if cleanup.Error() != reported || cleanup.Unwrap() != unwrapped || !errors.Is(cleanup, ast.ErrASTPending) {
		t.Fatal("nested qlinear cleanup changed stable failure identity")
	}
	if len(q.refs) != 0 || results[0] != (ast.ASTRef{}) {
		t.Fatal("nested qlinear retry left indexed roots", len(q.refs), results[0])
	}
	if _, err = owned.Head(); err == nil {
		t.Fatal("nested qlinear retry left rule owner open")
	}
}
