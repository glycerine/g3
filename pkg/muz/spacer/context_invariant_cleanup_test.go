package spacer

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

func TestSpacerInvariantPreservesFreshSolverAndRootsOnNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var attempt *invariantCheckAttempt
	var failureErr error
	var receipt memory.ConstructionFailure
	semantic := errors.New("PDR validation rejected result")
	active, callback, intact := false, false, true
	var runtime *memory.Runtime
	var err error
	runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && !callback {
			callback = true
			solverOwner := attempt.solver
			referenceOwner := attempt.references[0]
			failureErr = attempt.finish(semantic)
			intact = errors.Is(failureErr, semantic) && errors.Is(failureErr, ast.ErrASTPending) && errors.As(failureErr, &receipt) && receipt.RuntimeOwner() == runtime && attempt.solver == solverOwner && len(attempt.references) == 1 && attempt.references[0] == referenceOwner
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
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if receipt != nil {
			must(receipt.Cleanup())
		}
		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(parameters.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("PDR validation fixture leaked bytes", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("pending-pdr-validation")
	must(err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	application, err := manager.MkApp(declaration, nil)
	must(err)
	solver, err := smtcore.NewSMTSolver(manager, nil, "", smtcore.SMTSolverConfig{Params: parameters, Globals: globals})
	must(err)
	must(solver.Assert(application))
	reference, err := manager.NewRef(application)
	must(err)
	attempt = &invariantCheckAttempt{manager: manager, solver: solver}
	attempt.references = append(attempt.references, reference)
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || !callback || !intact || receipt == nil {
		t.Fatal("native refusal did not preserve validation owners", body, callback, intact, failureErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	origin, exposed, message := receipt.OriginalCause(), errors.Unwrap(receipt), receipt.Error()
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	if receipt.CleanupCause() != nil || attempt.solver != nil || len(attempt.references) != 0 {
		t.Fatal("validation cleanup retry did not finish")
	}
	if receipt.OriginalCause() != origin || errors.Unwrap(receipt) != exposed || receipt.Error() != message || !errors.Is(receipt, semantic) || !errors.Is(receipt, ast.ErrASTPending) {
		t.Fatal("validation cleanup retry changed its receipt")
	}
	receipt = nil
}
