package bmc

// Source: dl_bmc_engine.cpp qlinear::get_model proof-step publication.

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

func TestQuantifiedLinearProofTransitionPreservesNewOutputsWhileRetirementRefuses(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var manager *ast.ASTManager
	var next *base.Rule
	var current, proof ast.ASTRef
	var pending error
	var observedRuleErr error
	var observedRule *base.Rule
	var observedProof ast.ASTRef
	callbacks, intact := 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		callbacks++
		if pending == nil {
			publishedRule, publishedProof, cause := advanceQuantifiedLinearProofStep(manager, nil, next, current, proof, nil)
			pending = cause
			observedRule, observedProof = publishedRule, publishedProof
			_, observedRuleErr = next.Head()
			intact = publishedRule == nil && publishedProof == (ast.ASTRef{}) && errors.Is(cause, ast.ErrASTPending) && observedRuleErr == nil
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
			t.Fatal("quantified proof transition fixture leaked bytes", used)
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(manager)
	must(err)
	defer func() { must(rules.Close()) }()
	name, err := symbols.Intern("qlinear-proof-transition")
	must(err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(declaration))
	head, err := manager.MkApp(declaration, nil)
	must(err)
	rule, err := rules.NewRule(head, nil, nil, "qlinear-proof-transition")
	must(err)
	next, err = rule.Retain()
	must(err)
	must(rule.Close())
	current, err = manager.NewRef(head)
	must(err)
	proof, err = manager.NewRef(head)
	must(err)

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
		t.Fatal("refused quantified proof transition lost outputs", body, callbacks, intact, pending, observedRule, observedProof, observedRuleErr)
	}
	if _, err = manager.RefGet(proof); err != nil {
		t.Fatal("refused quantified proof transition consumed proof before retry", err)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(pending, &cleanup) {
		t.Fatal("refused quantified proof transition lost retry owner", pending)
	}
	reported := cleanup.Error()
	must(cleanup.Cleanup())
	if cleanup.Error() != reported || cleanup.Unwrap() == nil || !errors.Is(cleanup, ast.ErrASTPending) {
		t.Fatal("quantified proof transition lost stable failure identity")
	}
	if _, err = next.Head(); err == nil {
		t.Fatal("quantified proof transition retry left rule open")
	}
	if _, err = manager.RefGet(current); err == nil {
		t.Fatal("quantified proof transition retry left prior proof open")
	}
	if _, err = manager.RefGet(proof); err == nil {
		t.Fatal("quantified proof transition retry left new proof open")
	}
}
