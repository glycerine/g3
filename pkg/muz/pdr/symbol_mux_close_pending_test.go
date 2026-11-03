package pdr

// Source: sym_mux::m_ref_holder lifetime in
// src/muz/pdr/pdr_sym_mux.h:43-44.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSymbolMuxClosePreservesTupleRootsDuringNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var mux *SymbolMux
	var closeErr error
	active, callback := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && !callback {
			callback = true
			root := mux.references[len(mux.references)-1]
			closeErr = mux.Close()
			if !errors.Is(closeErr, ErrSymbolMuxOwner) || !errors.Is(closeErr, ast.ErrASTPending) || !mux.closing || mux.closed || mux.manager == nil || len(mux.references) == 0 || mux.references[len(mux.references)-1] != root {
				t.Fatalf("refused symbol-mux cleanup lost its tuple root: %v", closeErr)
			}
			if _, _, err := mux.TryIndex(ast.ASTHandle{}); !errors.Is(err, ast.ErrASTPending) {
				t.Fatalf("closing symbol mux accepted use: %v", err)
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
	mux, err = NewSymbolMux(manager)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	prefixName, err := symbols.Intern("pending-symbol-mux")
	must(err)
	prefix, err := manager.MkFuncDecl(prefixName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	tuple, err := mux.CreateTuple(prefix, []ast.ASTHandle{boolean}, boolean, 2, nil)
	must(err)
	if len(tuple) != 2 || len(mux.references) == 0 {
		t.Fatal("symbol mux did not own its source tuple")
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
	if body == nil || !callback || !errors.Is(closeErr, ast.ErrASTPending) || len(mux.references) == 0 {
		t.Fatal("native guard did not refuse symbol-mux cleanup", body, callback, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(mux.Close())
	must(mux.Close())
	if !mux.closed || mux.closing || mux.manager != nil || len(mux.references) != 0 {
		t.Fatal("symbol-mux cleanup retry did not finish")
	}

	must(numbers.Del(&value))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("symbol-mux cleanup fixture leaked bytes", budget.Used())
	}
}
