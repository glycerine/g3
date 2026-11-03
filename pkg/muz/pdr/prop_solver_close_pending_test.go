package pdr

// Source: prop_solver frame/core expression owners in
// src/muz/pdr/pdr_prop_solver.h:44-55.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPropSolverClosePreservesCoreAndFrameRootsDuringNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var solver *PropSolver
	var closeErr error
	active, callback := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && !callback {
			callback = true
			core := solver.coreReferences[0]
			frame := solver.references[0]
			closeErr = solver.Close()
			if !errors.Is(closeErr, ast.ErrASTPending) || !solver.closing || solver.closed || solver.manager == nil || len(solver.coreReferences) != 1 || solver.coreReferences[0] != core || len(solver.references) != 1 || solver.references[0] != frame {
				t.Fatalf("refused proposition-solver cleanup lost an AST owner: %v", closeErr)
			}
			if _, err := solver.UseFarkas(); !errors.Is(err, ast.ErrASTPending) {
				t.Fatalf("closing proposition solver accepted use: %v", err)
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
	boolean, err := manager.MkBoolSort()
	must(err)
	coreHandle, err := manager.MkFreshConst("pending-prop-core", boolean)
	must(err)
	frameHandle, err := manager.MkFreshConst("pending-prop-frame", boolean)
	must(err)
	core, err := manager.NewRef(coreHandle)
	must(err)
	frame, err := manager.NewRef(frameHandle)
	must(err)
	contexts := &SMTContextManager{active: 1}
	context := &SMTContext{parent: contexts, solver: &smtcore.SMTSolver{}}
	solver = &PropSolver{manager: manager, contexts: contexts, context: context, coreReferences: []ast.ASTRef{core}, references: []ast.ASTRef{frame}}

	value, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || !callback || !errors.Is(closeErr, ast.ErrASTPending) || !context.closed || contexts.active != 0 || len(solver.coreReferences) != 1 || len(solver.references) != 1 {
		t.Fatal("native guard did not refuse proposition-solver cleanup", body, callback, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(solver.Close())
	must(solver.Close())
	if !solver.closed || solver.closing || solver.manager != nil || solver.contexts != nil || solver.coreReferences != nil || solver.references != nil {
		t.Fatal("proposition-solver cleanup retry did not finish")
	}

	must(numbers.Del(&value))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("proposition-solver cleanup fixture leaked bytes", budget.Used())
	}
}
