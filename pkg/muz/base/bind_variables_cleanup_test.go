package base

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// The real arithmetic transport runs while the AST invocation rejects release
// admission. The binder's source m_vars roots must remain owned for retry.
func TestVariableBinderCloseRetriesNativePendingWithoutLosingRoots(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var binder *VariableBinder
	var input ast.ASTHandle
	active, called := false, false
	var references []ast.ASTRef
	var variables []ast.ASTHandle
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !called {
			called = true
			for i := 0; i < 2; i++ {
				if err := binder.Close(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("native cleanup refusal was lost", i, err)
				}
				if binder.closed || !binder.closing || !reflect.DeepEqual(binder.references, references) || !reflect.DeepEqual(binder.variables, variables) {
					t.Fatal("refused close discarded registered variable owners")
				}
				if err := binder.AddVariable(input); !errors.Is(err, ErrBindVariablesOwner) {
					t.Fatal("closing binder accepted registration", err)
				}
				if ref, err := binder.Bind(input, true); !errors.Is(err, ErrBindVariablesOwner) || ref != (ast.ASTRef{}) {
					t.Fatal("closing binder accepted binding", ref, err)
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
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	binder, err = NewVariableBinder(manager)
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		must(binder.Close())
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("variable binder cleanup leaked budget", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	for _, name := range []string{"binder-native-x", "binder-native-y"} {
		constant, err := manager.MkFreshConst(name, boolean)
		must(err)
		must(binder.AddVariable(constant))
	}
	variables = append([]ast.ASTHandle(nil), binder.variables...)
	references = append([]ast.ASTRef(nil), binder.references...)
	input = variables[0]
	body, err := manager.MkAnd(variables[0], variables[1])
	must(err)
	bound, err := binder.Bind(body, true)
	must(err)
	defer func() { must(manager.DelRef(&bound)) }()
	checkBound := func() {
		t.Helper()
		formula, err := manager.RefGet(bound)
		must(err)
		forall, err := manager.IsForall(formula)
		must(err)
		count, err := manager.GetNumDeclarations(formula)
		must(err)
		if !forall || count != 2 {
			t.Fatal("independently owned binding changed", forall, count)
		}
		quantified, err := manager.GetQuantifierBody(formula)
		must(err)
		for i, want := range []uint32{1, 0} {
			argument, err := manager.GetArg(quantified, uint32(i))
			must(err)
			index, err := manager.GetVarIndex(argument)
			must(err)
			if index != want {
				t.Fatal("binding changed source variable order", i, index, want)
			}
		}
	}
	checkBound()
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	before := manager.Stats()
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, failure := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if failure == nil || !called {
		t.Fatal("native transport did not exercise binder cleanup", failure, called)
	}
	afterRefusal := manager.Stats()
	if afterRefusal.LiveReferences != before.LiveReferences || afterRefusal.LiveNodes != before.LiveNodes || afterRefusal.LiveInfos != before.LiveInfos || afterRefusal.LiveParameters != before.LiveParameters {
		t.Fatal("native refusal changed live AST owners", before, afterRefusal)
	}
	must(binder.Close())
	must(binder.Close())
	if !binder.closed || binder.closing || len(binder.references) != 0 || len(binder.variables) != 0 || manager.Stats().LiveReferences+2 != before.LiveReferences {
		t.Fatal("retry did not release exactly the registered owners")
	}
	checkBound()
	for _, ref := range references {
		if _, err := manager.RefGet(ref); !errors.Is(err, ast.ErrASTHandle) {
			t.Fatal("registered variable reference survived completed cleanup", err)
		}
	}
}
