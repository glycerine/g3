package pdr

// Source: core_arith_inductive_generalizer-owned farkas_learner destruction
// in src/muz/pdr/pdr_farkas_learner.cpp.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestFarkasLearnerClosePreservesPremisesDuringNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var learner *FarkasLearner
	var retainedUtility interface{}
	var closeErr error
	active, callback := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && !callback {
			callback = true
			closeErr = learner.Close()
			if !errors.Is(closeErr, ast.ErrASTPending) || !learner.closing || learner.closed || learner.utility == nil || learner.utility != retainedUtility {
				t.Fatalf("refused Farkas cleanup lost its premise owner: %v", closeErr)
			}
			if err := learner.SetSplitLiterals(true); !errors.Is(err, ast.ErrASTHandle) {
				t.Fatalf("closing Farkas learner accepted use: %v", err)
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
	learner, err = NewFarkasLearner(manager, parameters, globals)
	must(err)
	arithmetic, err := manager.ArithmeticFamilyID()
	must(err)
	realSort, err := manager.MkFamilySort(arithmetic, 0, nil)
	must(err)
	x, err := manager.MkFreshConst("pending-farkas-premise", realSort)
	must(err)
	zero, err := manager.ArithmeticRealNumeralText("0")
	must(err)
	bound, err := manager.MkArithmeticLE(x, zero)
	must(err)
	must(learner.utility.Add(big.NewRat(1, 1), bound))
	retainedUtility = learner.utility

	value, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || !callback || !errors.Is(closeErr, ast.ErrASTPending) {
		t.Fatal("native guard did not refuse Farkas cleanup", body, callback, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(learner.Close())
	must(learner.Close())
	if !learner.closed || learner.closing || learner.manager != nil || learner.utility != nil {
		t.Fatal("Farkas cleanup retry did not finish")
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
		t.Fatal("Farkas cleanup fixture leaked bytes", budget.Used())
	}
}
