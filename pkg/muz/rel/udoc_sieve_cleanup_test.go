package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestUDocSieveCloseNativeRefusalSkipsConsumedSortPins(t *testing.T) {
	must := func(err error) { t.Helper(); astTestMust(t, err) }
	budget := arena.NewBudget(1 << 28)
	var sieve *UDocSieve
	var remaining []ast.ASTRef
	var failure error
	active, called := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && !called {
			called = true
			failure = sieve.Close()
			if !errors.Is(failure, ast.ErrASTPending) {
				t.Fatal("sieve Close did not reach native sort-pin refusal", failure)
			}
			for i := 0; i < 2; i++ {
				if err := sieve.Close(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("sieve Close retry changed native refusal", err)
				}
				if sieve.inner != nil || len(sieve.sortRefs) != len(remaining) || sieve.manager == nil {
					t.Fatal("sieve Close lost its remaining outer-sort frontier")
				}
				for index, ref := range remaining {
					if sieve.sortRefs[index] != ref {
						t.Fatal("native refusal consumed an earlier sort pin", index)
					}
				}
			}
		}
		return value
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
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
		if used := budget.Used(); used != 0 {
			t.Fatal("sieve native cleanup fixture leaked bytes", used)
		}
	}()
	factory, err := NewUDocRelationFactory(manager, arena.VectorConfig{Descriptors: arena.Config{Budget: budget}})
	must(err)
	defer factory.Close()
	guard, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&guard)) }()
	must(numbers.SetString(&guard, "18446744073709551617/18446744073709551619"))
	_, err = manager.ArithmeticNumeral(&guard, false)
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	unrelated, err := manager.NewRef(truth)
	must(err)
	defer func() { must(manager.DelRef(&unrelated)) }()
	baseline := manager.Stats().LiveReferences
	name, err := symbols.Intern("sieve-native-pending-ignored-sort")
	must(err)
	fresh, err := manager.MkUninterpretedSort(name, nil)
	must(err)
	sieve, err = NewFullUDocSieve(factory, []ast.ASTHandle{fresh, fresh, fresh})
	must(err)
	defer func() { must(sieve.Close()) }()
	// A completed trailing release is a consumed part of the reverse cleanup
	// frontier. Close must skip its zero token before reaching the native guard.
	must(manager.DelRef(&sieve.sortRefs[2]))
	remaining = append([]ast.ASTRef(nil), sieve.sortRefs[:2]...)
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&guard, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !called || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("native guard did not interrupt sieve cleanup", body, called, failure)
	}
	for _, ref := range remaining {
		got, err := manager.RefGet(ref)
		must(err)
		if got != fresh {
			t.Fatal("pending sieve cleanup changed its ignored sort", got)
		}
	}
	must(sieve.Close())
	must(sieve.Close())
	if len(sieve.sortRefs) != 0 || sieve.manager != nil || sieve.inner != nil || manager.Stats().LiveReferences != baseline {
		t.Fatal("sieve cleanup retry left sort owners", baseline, manager.Stats())
	}
	if manager.IsLiveHandle(fresh) {
		t.Fatal("completed sieve cleanup left its ignored sort alive")
	}
	for _, ref := range remaining {
		if _, err = manager.RefGet(ref); err == nil {
			t.Fatal("completed sieve cleanup left an outer-sort pin live")
		}
	}
	got, err := manager.RefGet(unrelated)
	must(err)
	if got != truth {
		t.Fatal("sieve cleanup consumed an unrelated root")
	}
}
