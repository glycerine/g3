package tab

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

var errTabProofInjected = errors.New("tab proof replay failed after construction")

func TestTabProofReplayOwnersSurviveNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var failure error
	var trigger func()
	active := false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && failure == nil && trigger != nil {
			trigger()
		}
		return source
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
	truth, err := manager.MkTrue()
	must(err)
	root, err := manager.NewRef(truth)
	must(err)
	result, err := manager.NewRef(truth)
	must(err)
	resolutionRef, err := manager.NewRef(truth)
	must(err)
	scope, err := ast.NewScopedProof(manager)
	must(err)
	converter, err := tactic.NewReplaceProofConverter(manager)
	must(err)
	proofAttempt := &tabProofAttempt{manager: manager, root: root, result: ast.ASTRef{}, converter: converter, scope: scope}
	trigger = func() {
		returned, finishErr := finishTabProofAttempt(proofAttempt, result, errTabProofInjected)
		if returned != (ast.ASTRef{}) {
			t.Fatal("failed proof replay published a result")
		}
		failure = finishErr
	}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	invoke := func() error {
		must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
		var invocation ast.ASTArithmeticNumeralInvocation
		active = true
		_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
		active = false
		must(runtime.SetMaxCount(0))
		must(manager.CleanupArithmeticNumeralInvocation(&invocation))
		return body
	}
	body := invoke()
	if body == nil || !errors.Is(failure, errTabProofInjected) || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("proof cleanup refusal not reached", body, failure)
	}
	owned, ok := failure.(*tabProofCleanupFailure)
	if !ok || owned.root != proofAttempt || proofAttempt.root != root || proofAttempt.result != result || proofAttempt.converter == nil || proofAttempt.scope == nil {
		t.Fatal("proof cleanup lost a replay owner")
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
		t.Fatal("proof failure is not retryable")
	}
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if proofAttempt.root != (ast.ASTRef{}) || proofAttempt.result != (ast.ASTRef{}) || proofAttempt.converter != nil || proofAttempt.scope != nil || cleanup.CleanupCause() != nil {
		t.Fatal("proof cleanup retry incomplete")
	}

	resolutionAttempt := &tabResolutionProofAttempt{manager: manager, refs: []ast.ASTRef{resolutionRef}}
	failure = nil
	trigger = func() { failure = finishTabResolutionProofAttempt(resolutionAttempt, errTabProofInjected) }
	body = invoke()
	if body == nil || !errors.Is(failure, errTabProofInjected) || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("resolution cleanup refusal not reached", body, failure)
	}
	owned, ok = failure.(*tabProofCleanupFailure)
	if !ok || owned.root != resolutionAttempt || len(resolutionAttempt.refs) != 1 || resolutionAttempt.refs[0] != resolutionRef {
		t.Fatal("resolution cleanup lost temporary refs")
	}
	cleanup = owned
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if len(resolutionAttempt.refs) != 0 || cleanup.CleanupCause() != nil {
		t.Fatal("resolution cleanup retry incomplete")
	}
}
