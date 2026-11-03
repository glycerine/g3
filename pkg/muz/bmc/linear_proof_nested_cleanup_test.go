package bmc

// Source: dl_bmc_engine.cpp linear::get_model nested substitution and
// normalization failures must finish before their enclosing proof-step roots.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestLinearProofStepCleanupDrainsNestedNativeFailureBeforeParent(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var manager *ast.ASTManager
	var owner *linearProofStepOwner
	var childRefs []ast.ASTRef
	var failure error
	operationCause := errors.New("proof normalization interrupted")
	active, callbacks := false, 0
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && failure == nil {
			callbacks++
			child := disposeASTRefs(manager, childRefs, operationCause)
			failure = owner.dispose(child)
		}
		return source
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
		if budget.Used() != 0 {
			t.Fatal("nested proof cleanup leaked bytes", budget.Used())
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	child, err := manager.NewRef(truth)
	must(err)
	childRefs = []ast.ASTRef{child}
	sub, err := rewriter.NewVarSubstitution(manager, false)
	must(err)
	ref, err := manager.NewRef(truth)
	must(err)
	proof, err := manager.NewRef(truth)
	must(err)
	owner = &linearProofStepOwner{manager: manager, sub: sub, refs: []ast.ASTRef{ref}, proof: proof}
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
	if body == nil || callbacks != 1 || !errors.Is(failure, ast.ErrASTPending) || !errors.Is(failure, operationCause) {
		t.Fatal("nested proof cleanup missed native refusal", body, callbacks, failure)
	}
	if owner.sub != sub || owner.refs[0] != ref || owner.proof != proof || childRefs[0] != child {
		t.Fatal("nested refusal advanced a dependent proof owner")
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
		t.Fatal("missing nested proof retry receipt", failure)
	}
	beforeMessage, beforeCause, beforeUnwrapped := cleanup.Error(), cleanup.OriginalCause(), cleanup.Unwrap()
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if childRefs[0] != (ast.ASTRef{}) {
		// Release the orphan only to keep a failing pre-fix regression's
		// fixture teardown valid; the assertion still exposes the lost child.
		must(manager.DelRef(&childRefs[0]))
		t.Fatal("proof retry discarded its nested cleanup obligation")
	}
	if owner.sub != nil || owner.refs[0] != (ast.ASTRef{}) || owner.proof != (ast.ASTRef{}) {
		t.Fatal("nested proof cleanup retry left parent owners")
	}
	if cleanup.Error() != beforeMessage || cleanup.OriginalCause() != beforeCause || cleanup.Unwrap() != beforeUnwrapped || cleanup.CleanupCause() != nil {
		t.Fatal("nested proof cleanup changed original diagnostics")
	}
}
