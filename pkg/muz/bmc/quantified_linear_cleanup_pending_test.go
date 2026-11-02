package bmc

// Source: dl_bmc_engine.cpp qlinear temporary expr_ref ownership.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifiedLinearCleanupPreservesRefsWhileManagerExecutes(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var q *quantifiedLinear
	var results []ast.ASTRef
	var refused error
	callbacks, intact := 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		callbacks++
		before := append([]ast.ASTRef(nil), q.refs...)
		beforeResult := results[0]
		refused = q.disposeResults(results, nil)
		intact = intact && errors.Is(refused, ast.ErrASTPending) && len(q.refs) == len(before) && results[0] == beforeResult
		for i := range before {
			intact = intact && q.refs[i] == before[i]
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if used := budget.Used(); used != 0 {
			t.Fatal("quantified cleanup fixture leaked bytes", used)
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	q, err = newQuantifiedLinear(m, 4)
	must(err)
	if len(q.refs) != 3 {
		t.Fatal("quantified owner roots", len(q.refs))
	}
	result, err := m.NewRef(q.one)
	must(err)
	results = []ast.ASTRef{result}

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("refused qlinear cleanup lost roots", body, callbacks, intact)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(refused, &cleanup) {
		t.Fatal("refused qlinear cleanup lost retry owner", refused)
	}
	must(cleanup.Cleanup())
	if cleanup.Error() == "" || cleanup.Unwrap() == nil || !errors.Is(cleanup, ast.ErrASTPending) {
		t.Fatal("qlinear cleanup retry lost original refusal", cleanup.Error(), cleanup.Unwrap())
	}
	if len(q.refs) != 0 {
		t.Fatal("qlinear cleanup retry left roots", len(q.refs))
	}
	if results[0] != (ast.ASTRef{}) {
		t.Fatal("qlinear cleanup retry left published result")
	}
	must(q.dispose())
}
