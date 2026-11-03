package pdr

// Source: manager-owned sym_mux, SMT-context pool and background lifetime in
// src/muz/pdr/pdr_manager.h:77-87.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestManagerClosePreservesCompositeOwnersDuringNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var owner *Manager
	var closeErr error
	active, callback := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && !callback {
			callback = true
			background := owner.background
			mux := owner.mux
			contexts := owner.contexts
			muxRoots := len(mux.references)
			muxLast := mux.references[muxRoots-1]
			closeErr = owner.Close()
			if !errors.Is(closeErr, ast.ErrASTPending) || !owner.closing || owner.closed || owner.manager == nil || owner.mux != mux || owner.contexts != contexts || !contexts.closed || owner.background != background || mux.closing || mux.closed || len(mux.references) != muxRoots || mux.references[muxRoots-1] != muxLast {
				t.Fatalf("refused PDR-manager cleanup lost a composite owner: %v", closeErr)
			}
			if _, err := owner.Background(); !errors.Is(err, ast.ErrASTPending) {
				t.Fatalf("closing PDR manager accepted use: %v", err)
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
	parameters, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, parameters, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	owner, err = NewManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: parameters, Globals: globals})
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	prefixName, err := symbols.Intern("pending-pdr-manager")
	must(err)
	prefix, err := manager.MkFuncDecl(prefixName, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	if _, err = owner.NewPredicate(prefix); err != nil || len(owner.mux.references) == 0 {
		t.Fatal("PDR manager did not construct its source state tuple", err)
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
	if body == nil || !callback || !errors.Is(closeErr, ast.ErrASTPending) || owner.mux == nil || owner.contexts == nil {
		t.Fatal("native guard did not refuse PDR-manager cleanup", body, callback, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(owner.Close())
	must(owner.Close())
	if !owner.closed || owner.closing || owner.manager != nil || owner.mux != nil || owner.contexts != nil || owner.background != (ast.ASTRef{}) {
		t.Fatal("PDR-manager cleanup retry did not finish")
	}

	must(numbers.Del(&value))
	must(manager.Close())
	must(globals.Close())
	must(catalog.Close())
	must(parameters.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("PDR-manager cleanup fixture leaked bytes", budget.Used())
	}
}
