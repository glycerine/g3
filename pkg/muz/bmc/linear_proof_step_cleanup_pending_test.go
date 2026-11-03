package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp linear::get_model proof-step owner
// cleanup across native AST refusal.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestLinearProofStepCleanupPreservesAllOwnersAcrossNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var manager *ast.ASTManager
	var owner *linearProofStepOwner
	var failure error
	callbacks, intact := 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		callbacks++
		beforeRef, beforeProof := owner.refs[0], owner.proof
		beforeTemporary, beforeNext := owner.temps[0], owner.next
		failure = owner.dispose(nil)
		intact = intact && errors.Is(failure, ast.ErrASTPending) &&
			owner.refs[0] == beforeRef && owner.proof == beforeProof &&
			owner.temps[0] == beforeTemporary && owner.next == beforeNext
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
			t.Fatal("linear proof-step cleanup fixture leaked bytes", used)
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(manager)
	must(err)
	defer func() { must(rules.Close()) }()
	name, err := symbols.Intern("linear-step-cleanup-goal")
	must(err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(declaration))
	head, err := manager.MkApp(declaration, nil)
	must(err)
	temporary, err := rules.NewRule(head, nil, nil, "linear-step-temporary")
	must(err)
	next, err := rules.NewRule(head, nil, nil, "linear-step-next")
	must(err)
	unifier, err := transforms.NewRuleUnifier(manager)
	must(err)
	sub, err := rewriter.NewVarSubstitution(manager, false)
	must(err)
	ref, err := manager.NewRef(head)
	must(err)
	proof, err := manager.NewRef(head)
	must(err)
	beforeMode, err := manager.ProofMode()
	must(err)
	scope, err := ast.NewScopedProof(manager)
	must(err)
	owner = &linearProofStepOwner{
		manager: manager,
		scope:   scope,
		refs:    []ast.ASTRef{ref},
		temps:   []*base.Rule{temporary},
		sub:     sub,
		unifier: unifier,
		next:    next,
		proof:   proof,
	}

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
		t.Fatal("refused linear proof-step cleanup lost an owner", body, callbacks, intact)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) {
		t.Fatal("linear proof-step cleanup lost concrete retry owner", failure)
	}
	pendingMode, err := manager.ProofMode()
	must(err)
	if pendingMode != ast.ASTProofEnabled || owner.scope == nil {
		t.Fatal("pending cleanup lost its active proof scope")
	}
	must(cleanup.Cleanup())
	afterMode, err := manager.ProofMode()
	must(err)
	if afterMode != beforeMode || owner.scope != nil {
		t.Fatal("cleanup failed to restore proof mode", beforeMode, afterMode)
	}
	if cleanup.Error() == "" || !errors.Is(cleanup.Unwrap(), ast.ErrASTPending) {
		t.Fatal("linear proof-step cleanup lost stable refusal cause")
	}
	if owner.refs[0] != (ast.ASTRef{}) || owner.temps[0] != nil || owner.sub != nil || owner.unifier != nil || owner.next != nil || owner.proof != (ast.ASTRef{}) {
		t.Fatal("linear proof-step cleanup retry left owners")
	}
	if err = owner.dispose(nil); err != nil {
		t.Fatal("linear proof-step cleanup is not idempotent", err)
	}
	if _, err = temporary.Head(); err == nil {
		t.Fatal("linear temporary rule remained open")
	}
	if _, err = next.Head(); err == nil {
		t.Fatal("linear next rule remained open")
	}
}
