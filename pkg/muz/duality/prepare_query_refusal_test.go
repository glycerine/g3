package duality

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// The source's local expr_ref_vector owns both the rule clauses and background
// inputs. A refused release keeps the prepared query's roots until retry.
func TestPreparedQueryClosePreservesRefusedRoots(t *testing.T) {
	for _, background := range []bool{false, true} {
		name := "clause"
		if background {
			name = "background"
		}
		t.Run(name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 28)
			var prepared *PreparedQuery
			var refusal error
			active, callback := false, false
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
				if active && !callback {
					callback = true
					oldClauses, oldBackground := len(prepared.clauses), len(prepared.background)
					refusal = prepared.Close()
					if !errors.Is(refusal, ast.ErrASTPending) || !prepared.closing || len(prepared.clauses) != oldClauses || len(prepared.background) != oldBackground {
						t.Fatalf("refused close lost an owning root: %v", refusal)
					}
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
			must(manager.RegisterDeclarationPlugins())
			truth, err := manager.MkTrue()
			must(err)
			boolean, err := manager.MkBoolSort()
			must(err)
			formula, err := manager.MkFreshConst("prepared-query-refused", boolean)
			must(err)
			if background {
				prepared, err = PrepareQuery(manager, nil, nil, truth, nil, 0)
				must(err)
				must(prepared.AddBackground(formula))
			} else {
				prepared, err = PrepareQuery(manager, []ast.ASTHandle{formula}, []uint32{0}, truth, nil, 0)
				must(err)
			}
			value, err := numbers.NewValue()
			must(err)
			must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
			must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
			var invocation ast.ASTArithmeticNumeralInvocation
			active = true
			_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
			active = false
			must(runtime.SetMaxCount(0))
			if body == nil || !callback || !errors.Is(refusal, ast.ErrASTPending) {
				t.Fatal("native guard did not refuse cleanup", body, refusal)
			}
			if _, err := prepared.Clauses(); !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("closing query exposed clauses", err)
			}
			if _, err := prepared.Bounds(); !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("closing query exposed bounds", err)
			}
			if err := prepared.AddBackground(truth); !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("closing query accepted background", err)
			}
			must(manager.CleanupArithmeticNumeralInvocation(&invocation))
			must(prepared.Close())
			must(prepared.Close())
			must(numbers.Del(&value))
			must(manager.Close())
			must(numbers.Close())
			must(symbols.Close())
			must(runtime.Close())
			if budget.Used() != 0 {
				t.Fatal("prepared-query cleanup leaked", budget.Used())
			}
		})
	}
}
