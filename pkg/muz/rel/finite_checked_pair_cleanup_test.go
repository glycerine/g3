package rel

// Source: dl_check_table.cpp check_table::~check_table releases the primary
// and checker owners in order. Go native refusal must preserve that frontier.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestFiniteCheckedPairClosePreservesNativeRefusalFrontier(t *testing.T) {
	for _, kinds := range []struct{ primary, checker string }{
		{"lazy_sparse", "sparse"}, {"sparse", "lazy_sparse"}, {"lazy_sparse", "lazy_sparse"},
	} {
		t.Run(kinds.primary+"/"+kinds.checker, func(t *testing.T) {
			budget := arena.NewBudget(1 << 28)
			var pair *FiniteCheckedPair
			var first, second error
			active, inside := false, false
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
				if active && !inside && first == nil {
					inside = true
					first = pair.Close()
					second = pair.Close()
					inside = false
				}
				return failure
			}})
			must(t, err)
			numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
				Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
				Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
			}})
			must(t, err)
			symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
			must(t, err)
			manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
			must(t, err)
			_, err = manager.RegisterArithmeticPlugin()
			must(t, err)
			relations, err := NewRelationManager(manager)
			must(t, err)
			number, err := numbers.NewValue()
			must(t, err)
			must(t, numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
			t.Cleanup(func() {
				must(t, relations.Close())
				must(t, numbers.Del(&number))
				must(t, manager.Close())
				must(t, numbers.Close())
				must(t, symbols.Close())
				must(t, runtime.Close())
				if used := budget.Used(); used != 0 {
					t.Fatal("checked native cleanup fixture leaked bytes", used)
				}
			})
			theory := tableRelationTheoryRewriter(t, manager)
			lazy := NewLazySparseFiniteFactory()
			t.Cleanup(func() {
				if pair != nil {
					must(t, pair.Close())
				}
				must(t, lazy.Close())
			})
			factory, err := NewFiniteCheckedFactoryWithLazy(kinds.primary, kinds.checker, lazy)
			must(t, err)
			pair, err = factory.New(lazySignature(t, 2))
			must(t, err)
			_, err = pair.Add(Fact{1})
			must(t, err)
			boolean, err := manager.MkBoolSort()
			must(t, err)
			truth, err := manager.MkTrue()
			must(t, err)
			must(t, pair.FilterInterpreted(relations, []ast.ASTHandle{boolean}, theory, []ast.ASTHandle{truth}, nil, nil))
			primary, checker := pair.primary, pair.checker
			var conditionRefs uint64
			for _, side := range []FiniteTable{primary, checker} {
				if concrete, ok := side.(*LazySparseTable); ok {
					if concrete.root.interpreted == nil || concrete.root.interpreted.owner == nil {
						t.Fatal("fixture has no actual interpreted owner")
					}
					conditionRefs += uint64(len(concrete.root.interpreted.owner.refs))
				}
			}
			before := manager.Stats().LiveReferences
			must(t, runtime.SetMaxCount(uint64(runtime.State().GlobalCount)+1))
			active = true
			var invocation ast.ASTArithmeticNumeralInvocation
			_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
			active = false
			must(t, runtime.SetMaxCount(0))
			must(t, manager.CleanupArithmeticNumeralInvocation(&invocation))
			if body == nil || !errors.Is(first, ast.ErrASTPending) || !errors.Is(second, ast.ErrASTPending) || pair.closed {
				t.Fatal("native refusal falsely completed close", body, first, second, pair.closed)
			}
			if pair.checker != checker || kinds.primary == "lazy_sparse" && pair.primary != primary || kinds.primary == "sparse" && pair.primary != nil {
				t.Fatal("close lost or repeated a concrete owner", pair.primary, pair.checker)
			}
			if refs := manager.Stats().LiveReferences; refs != before {
				t.Fatal("native refusal consumed an interpreted condition", before, refs)
			}
			must(t, pair.Close())
			must(t, pair.Close())
			if !pair.closed || pair.primary != nil || pair.checker != nil || uint64(manager.Stats().LiveReferences)+conditionRefs != uint64(before) {
				t.Fatal("retry left a concrete table or condition owner", pair.closed, manager.Stats().LiveReferences, before, conditionRefs)
			}
			if _, err := pair.Facts(); !errors.Is(err, ErrCheckTable) {
				t.Fatal("closed pair accepted a data operation", err)
			}
		})
	}
}
