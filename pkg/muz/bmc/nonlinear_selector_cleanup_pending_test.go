package bmc

// Source: dl_bmc_engine.cpp nonlinear::get_proof selector temporaries.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestNonlinearSelectorCleanupPreservesRootsAndNestedReceipt(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "nested"}[nested], func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 28)
			var manager *ast.ASTManager
			var refs, childRefs []ast.ASTRef
			var failure error
			callbacks, intact := 0, true
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
				callbacks++
				if failure == nil {
					before := append([]ast.ASTRef(nil), refs...)
					var child error
					if nested {
						child = disposeASTRefs(manager, childRefs, nil)
					}
					var matched bool
					matched, failure = finishNonlinearSelector(manager, refs, true, child)
					intact = !matched && errors.Is(failure, ast.ErrASTPending)
					for i := range refs {
						intact = intact && refs[i] == before[i]
					}
					if nested {
						owner, ok := failure.(*astRefCleanupFailure)
						intact = intact && ok && owner.child == child && childRefs[0] != (ast.ASTRef{})
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
					t.Fatal("nonlinear selector cleanup leaked bytes", used)
				}
			}()
			_, err = manager.RegisterArithmeticPlugin()
			must(err)
			truth, err := manager.MkTrue()
			must(err)
			declaration, err := manager.GetDecl(truth)
			must(err)
			for _, value := range []ast.ASTHandle{declaration, truth, truth} {
				ref, err := manager.NewRef(value)
				must(err)
				refs = append(refs, ref)
			}
			if nested {
				child, err := manager.NewRef(truth)
				must(err)
				childRefs = []ast.ASTRef{child}
			}
			number, err := numbers.NewValue()
			must(err)
			defer func() { must(numbers.Del(&number)) }()
			must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
			must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
			var invocation ast.ASTArithmeticNumeralInvocation
			_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
			must(runtime.SetMaxCount(0))
			must(manager.CleanupArithmeticNumeralInvocation(&invocation))
			if body == nil || callbacks == 0 || !intact {
				t.Fatal("refused selector cleanup lost roots or published a match", body, callbacks, intact)
			}
			var cleanup memory.ConstructionFailure
			if !errors.As(failure, &cleanup) {
				t.Fatal("selector cleanup lost retry receipt", failure)
			}
			reported, unwrapped := cleanup.Error(), cleanup.Unwrap()
			must(cleanup.Cleanup())
			must(cleanup.Cleanup())
			if cleanup.Error() != reported || cleanup.Unwrap() != unwrapped || !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatal("selector cleanup changed stable refusal identity")
			}
			for _, ref := range append(refs, childRefs...) {
				if ref != (ast.ASTRef{}) {
					t.Fatal("selector cleanup retry left an owner")
				}
			}
		})
	}
}
