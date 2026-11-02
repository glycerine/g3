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
	testProjectionPendingOwner(t, "combined")
}

func TestLegacyArrayProjectionOwnersPreservePendingReferences(t *testing.T) {
	for _, kind := range []string{"equalities", "formula", "selects"} {
		t.Run(kind, func(t *testing.T) { testProjectionPendingOwner(t, kind) })
	}
}

func testProjectionPendingOwner(t *testing.T, kind string) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var combined *LegacyProjectionResult
	var closeOwner func() error
	var intactOwner func() bool
	active, callbacks, intact := false, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				closeErr := closeOwner()
				intact = intact && errors.Is(closeErr, ast.ErrASTPending) && intactOwner()
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
	switch kind {
	case "combined":
		result := &LegacyProjectionResult{manager: m, formula: formulaRef, variables: []ast.ASTRef{variableRef}}
		combined = result
		closeOwner = result.Close
		intactOwner = func() bool {
			return result.manager == m && result.formula == formulaRef && len(result.variables) == 1 && result.variables[0] == variableRef
		}
	case "formula":
		result := &legacyArrayProjectionResult{manager: m, formula: formulaRef, variables: []ast.ASTRef{variableRef}}
		closeOwner = result.Close
		intactOwner = func() bool {
			return result.manager == m && result.formula == formulaRef && len(result.variables) == 1 && result.variables[0] == variableRef
		}
	case "selects":
		result := &legacyArraySelectProjection{manager: m, formula: formulaRef, variables: []ast.ASTRef{variableRef}}
		closeOwner = result.Close
		intactOwner = func() bool {
			return result.manager == m && result.formula == formulaRef && len(result.variables) == 1 && result.variables[0] == variableRef
		}
	case "equalities":
		result := &legacyArrayEqualities{manager: m, equalities: []ast.ASTRef{variableRef, formulaRef}, stores: map[ast.ASTHandle]bool{variable: true}}
		closeOwner = result.Close
		intactOwner = func() bool {
			return result.manager == m && len(result.equalities) == 2 && result.equalities[0] == variableRef && result.equalities[1] == formulaRef && result.stores[variable]
		}
	default:
		t.Fatal("unknown projection owner", kind)
	}
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
	got, err := m.RefGet(formulaRef)
	must(err)
	gotVariable, err := m.RefGet(variableRef)
	must(err)
	if got != formula || gotVariable != variable {
		t.Fatal("residual projection changed after refusal")
	}
	if combined != nil {
		h, err := combined.Formula()
		must(err)
		vars, err := combined.Variables()
		must(err)
		if h != formula || len(vars) != 1 || vars[0] != variable {
			t.Fatal("combined result access changed")
		}
	}
	must(closeOwner())
	must(m.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("projection result leaked bytes", budget.Used())
	}
}
