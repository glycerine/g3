package pdr

// Source: reachable_cache ref-holder destruction in
// src/muz/pdr/pdr_reachable_cache.cpp.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestReachableHashCacheClosePreservesUnconsumedRoots(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var cache *ReachableCache
	var cube ast.ASTHandle
	var cached ast.ASTRef
	var closeErr error
	active, callback := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && !callback {
			callback = true
			closeErr = cache.Close()
			if !errors.Is(closeErr, ast.ErrASTPending) || cache.closed || cache.manager == nil || len(cache.cache) != 1 || cache.cache[cube] != cached {
				t.Fatalf("refused reachable-cache cleanup lost its root: %v", closeErr)
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
	cache, err = NewReachableCache(manager, &SMTContextManager{}, truth, ReachableHashCache)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	cube, err = manager.MkFreshConst("reachable-close-pending", boolean)
	must(err)
	must(cache.AddReachable(cube))
	if len(cache.cache) != 1 {
		t.Fatal("hash cache did not retain its source cube")
	}
	cached = cache.cache[cube]

	value, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || !callback || !errors.Is(closeErr, ast.ErrASTPending) || len(cache.cache) != 1 {
		t.Fatal("native guard did not preserve reachable-cache cleanup", body, callback, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(cache.Close())
	must(cache.Close())
	if !cache.closed || cache.manager != nil || len(cache.cache) != 0 {
		t.Fatal("reachable-cache cleanup retry did not finish")
	}

	must(numbers.Del(&value))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("reachable-cache cleanup fixture leaked bytes", budget.Used())
	}
}
