package bmc

// Source: dl_bmc_engine.cpp nonlinear::bind_vars lexical destruction.

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

func TestNonlinearBindingCleanupPreservesOwnersAcrossNativeRefusal(t *testing.T) {
	for _, stage := range []string{"nested", "substitution", "temporaries", "result"} {
		t.Run(stage, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 28)
			var manager *ast.ASTManager
			var owner *nonlinearBindingOwner
			var childRefs []ast.ASTRef
			var failure error
			var published ast.ASTRef
			var operationCause error
			if stage == "nested" || stage == "result" {
				operationCause = errors.New("nonlinear binding failed before publication")
			}
			active, callbacks := false, 0
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
				if active && failure == nil {
					callbacks++
					cause := operationCause
					if stage == "nested" {
						cause = disposeASTRefs(manager, childRefs, cause)
					}
					published, failure = finishNonlinearBinding(owner, cause)
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
					t.Fatal("nonlinear binding cleanup leaked bytes", budget.Used())
				}
			}()
			_, err = manager.RegisterArithmeticPlugin()
			must(err)
			owner = &nonlinearBindingOwner{manager: manager}
			boolean, err := manager.MkBoolSort()
			must(err)
			variable, err := manager.MkVar(0, boolean)
			must(err)
			truth, err := manager.MkTrue()
			must(err)
			owner.result, err = manager.NewRef(truth)
			must(err)
			if stage != "result" {
				for _, value := range []ast.ASTHandle{variable, truth} {
					ref, err := manager.NewRef(value)
					must(err)
					owner.refs = append(owner.refs, ref)
				}
			}
			if stage == "nested" || stage == "substitution" {
				owner.substitution, err = rewriter.NewVarSubstitution(manager, false)
				must(err)
				must(owner.substitution.Substitute(variable, []ast.ASTHandle{truth}, owner.refs[1]))
			}
			if stage == "nested" {
				ref, err := manager.NewRef(truth)
				must(err)
				childRefs = []ast.ASTRef{ref}
			}
			beforeSub, beforeResult := owner.substitution, owner.result
			beforeRefs := append([]ast.ASTRef(nil), owner.refs...)
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
			if body == nil || callbacks == 0 || !errors.Is(failure, ast.ErrASTPending) || published != (ast.ASTRef{}) {
				t.Fatal("native refusal did not suppress binding publication", body, callbacks, failure, published)
			}
			if operationCause != nil && !errors.Is(failure, operationCause) {
				t.Fatal("binding cleanup lost operation cause", failure)
			}
			if owner.substitution != beforeSub || owner.result != beforeResult || len(owner.refs) != len(beforeRefs) {
				t.Fatal("refusal advanced a dependent binding owner")
			}
			for i := range beforeRefs {
				if owner.refs[i] != beforeRefs[i] {
					t.Fatal("refusal consumed a binding temporary", i)
				}
			}
			if stage == "nested" && (owner.nested == nil || childRefs[0] == (ast.ASTRef{})) {
				t.Fatal("refusal lost nested binding owner")
			}
			var cleanup memory.ConstructionFailure
			if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
				t.Fatal("binding cleanup has no retry receipt", failure)
			}
			original, exposed, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
			must(cleanup.Cleanup())
			must(cleanup.Cleanup())
			if owner.substitution != nil || len(owner.refs) != 0 || owner.result != (ast.ASTRef{}) || owner.nested != nil {
				t.Fatal("binding cleanup retry left an owner")
			}
			if stage == "nested" && childRefs[0] != (ast.ASTRef{}) {
				t.Fatal("binding cleanup retry left nested root")
			}
			if cleanup.OriginalCause() != original || errors.Unwrap(cleanup) != exposed || cleanup.Error() != message || cleanup.CleanupCause() != nil {
				t.Fatal("binding cleanup retry changed failure diagnostics")
			}
		})
	}
}
