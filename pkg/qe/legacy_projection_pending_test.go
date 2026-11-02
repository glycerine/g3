package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Exercise the result owner independently of which theory leaves variables
// unprojected: both the residual formula and remaining variable own AST roots.
func TestLegacyProjectionResultClosePreservesPendingOwners(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var result *LegacyProjectionResult
	var mOwner *ast.ASTManager
	active, callbacks, intact := false, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			formula, variable := result.formula, result.variables[0]
			for i := 0; i < 2; i++ {
				closeErr := result.Close()
				intact = intact && errors.Is(closeErr, ast.ErrASTPending) && result.manager == mOwner && result.formula == formula && len(result.variables) == 1
				if len(result.variables) == 1 {
					intact = intact && result.variables[0] == variable
				}
			}
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

	mOwner = m
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	variable, err := m.MkFreshConst("residual-variable", boolean)
	must(err)
	variableRef, err := m.NewRef(variable)
	must(err)
	formula, err := m.MkNot(variable)
	must(err)
	formulaRef, err := m.NewRef(formula)
	must(err)
	result = &LegacyProjectionResult{manager: m, formula: formulaRef, variables: []ast.ASTRef{variableRef}}
	value, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	must(numbers.Del(&value))
	if callbacks == 0 || body == nil {
		t.Fatal("did not reach numerical transport refusal")
	}
	if !intact {
		t.Fatal("projection result discarded refused owners")
	}
	got, err := result.Formula()
	must(err)
	vars, err := result.Variables()
	must(err)
	if got != formula || len(vars) != 1 || vars[0] != variable {
		t.Fatal("residual projection changed after refusal")
	}
	must(result.Close())
	must(m.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("projection result leaked bytes", budget.Used())
	}
}
