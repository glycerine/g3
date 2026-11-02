package clp

// Source: src/muz/clp/clp_context.cpp imp::m_ground/reset_ground ownership.
// Rule-local fresh constants stay owned until expr_ref_vector cleanup succeeds.

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

func TestCLPGroundCleanupPreservesFreshConstantsOnNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var environment *groundEnvironment
	var failure error
	var result ast.ASTRef
	var refuse func() error
	active := false
	callbacks := 0
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && callbacks == 0 {
			callbacks++
			failure = refuse()
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("fixture leaked bytes", budget.Used())
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	fresh, err := manager.MkFreshConst("clp-ground-owner", boolean)
	must(err)
	ref, err := manager.NewRef(fresh)
	must(err)
	original := ref
	result, err = manager.CopyRef(ref)
	must(err)
	originalResult := result
	environment = &groundEnvironment{manager: manager, values: []ast.ASTHandle{fresh}, refs: []ast.ASTRef{ref}}
	refuse = func() error { return environment.disposeResult(&result, false) }

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("did not reach native cleanup refusal", body, failure)
	}
	if result != (ast.ASTRef{}) || len(environment.refs) != 2 || environment.refs[0] != original || environment.refs[1] != originalResult || environment.values[0] != fresh {
		t.Fatal("refused cleanup lost its fresh-constant owner")
	}
	for i := range environment.refs {
		if _, err := manager.RefGet(environment.refs[i]); err != nil {
			t.Fatal("refused cleanup invalidated its owner", i, err)
		}
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) {
		t.Fatal("cleanup refusal is not retryable", failure)
	}
	must(cleanup.Cleanup())
	if environment.refs != nil || environment.values != nil {
		t.Fatal("cleanup retry did not retire grounding owners")
	}

	variable, err := manager.MkVar(0, boolean)
	must(err)
	attemptEnvironment := &groundEnvironment{manager: manager}
	grounded, err := attemptEnvironment.ground(variable)
	must(err)
	owner := &ruleAttemptOwner{manager: manager, environment: attemptEnvironment}
	owner.take(grounded)
	originalGrounded := grounded
	originalFresh := append([]ast.ASTRef(nil), attemptEnvironment.refs...)
	failure = nil
	callbacks = 0
	refuse = owner.dispose
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	invocation = ast.ASTArithmeticNumeralInvocation{}
	active = true
	_, body = manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("did not reach rule-attempt cleanup refusal", body, failure)
	}
	if len(owner.refs) != 1 || owner.refs[0] != originalGrounded || len(attemptEnvironment.refs) != len(originalFresh) {
		t.Fatal("refused rule-attempt cleanup lost an owner")
	}
	for i := range originalFresh {
		if attemptEnvironment.refs[i] != originalFresh[i] {
			t.Fatal("refused rule-attempt cleanup changed fresh owner", i)
		}
	}
	if _, err := manager.RefGet(owner.refs[0]); err != nil {
		t.Fatal("refused rule-attempt cleanup invalidated grounded result", err)
	}
	cleanup = nil
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
		t.Fatal("rule-attempt cleanup refusal is not retryable", failure)
	}
	must(cleanup.Cleanup())
	if owner.refs != nil || attemptEnvironment.refs != nil || attemptEnvironment.values != nil || cleanup.CleanupCause() != nil {
		t.Fatal("rule-attempt cleanup retry did not retire all owners")
	}

	ruleManager, err := base.NewRuleManager(manager)
	must(err)
	defer func() { must(ruleManager.Close()) }()
	name, err := symbols.Intern("clp-candidate-owner")
	must(err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(ruleManager.RegisterPredicate(declaration))
	application, err := manager.MkApp(declaration, nil)
	must(err)
	rule, err := ruleManager.NewRule(application, nil, nil, "candidate")
	must(err)
	set, err := base.NewRuleSet(ruleManager)
	must(err)
	defer func() { must(set.Close()) }()
	must(set.Add(rule))
	must(rule.Close())
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("candidate fixture is not stratified")
	}
	candidates, err := sortedRules(set, declaration)
	must(err)
	failure = nil
	callbacks = 0
	refuse = func() error { return closeRules(candidates) }
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	invocation = ast.ASTArithmeticNumeralInvocation{}
	active = true
	_, body = manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks != 1 || failure != nil {
		t.Fatal("shared candidate release reached native cleanup", body, callbacks, failure)
	}
	originalRule, err := set.Rule(0)
	must(err)
	if head, err := originalRule.Head(); err != nil || head != application {
		t.Fatal("candidate release changed the program rule", head, err)
	}
	must(originalRule.Close())
}
