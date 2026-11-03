package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerInductiveExprExportPreservesCopiedResultOnNativeCleanupRefusal(t *testing.T) {
	t.Run("successful conversion", func(t *testing.T) { testSpacerInductiveExprExportCleanup(t, nil) })
	t.Run("failed conversion", func(t *testing.T) { testSpacerInductiveExprExportCleanup(t, errors.New("cover conversion failed")) })
}
func testSpacerInductiveExprExportCleanup(t *testing.T, operation error) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var attempt *inductiveExprExportAttempt
	var failureErr error
	var receipt memory.ConstructionFailure
	var failedResult ast.ASTRef
	active, callback, intact := false, false, true
	var runtime *memory.Runtime
	var err error
	runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && !callback {
			callback = true
			resultOwner := attempt.result
			modelOwner := attempt.model
			failedResult, failureErr = attempt.finish(operation)
			intact = failedResult == (ast.ASTRef{}) && errors.Is(failureErr, ast.ErrASTPending) && errors.As(failureErr, &receipt) && receipt.RuntimeOwner() == runtime && attempt.result == resultOwner && attempt.model == modelOwner && attempt.builder != nil
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
			t.Fatal("expression export fixture leaked bytes", budget.Used())
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
	md, err := model.NewModel(manager)
	must(err)
	must(md.Core().RegisterConstant(declaration, application))
	builder, err := model.NewModel2ExprBuilder(md.Core())
	must(err)
	output, err := manager.NewRef(ast.ASTHandle{})
	must(err)
	must(builder.Build(output))
	attempt = &inductiveExprExportAttempt{manager: manager, runtime: runtime, model: md, builder: builder, result: output}
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
		t.Fatal("native refusal did not preserve expression owners", body, callback, intact, failureErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	origin, exposed, message := receipt.OriginalCause(), errors.Unwrap(receipt), receipt.Error()
	if operation != nil && (origin != operation || !errors.Is(receipt, operation)) {
		t.Fatal("cleanup obscured the original operation", origin, operation)
	}
	if operation == nil && origin != ast.ErrASTPending {
		t.Fatal("cleanup-only original cause changed", origin)
	}

	must(receipt.Cleanup())
	must(receipt.Cleanup())
	if receipt.CleanupCause() != nil || attempt.result != (ast.ASTRef{}) || attempt.model != nil || attempt.builder != nil {
		t.Fatal("expression cleanup retry did not finish")
	}
	if receipt.OriginalCause() != origin || errors.Unwrap(receipt) != exposed || receipt.Error() != message || !errors.Is(receipt, ast.ErrASTPending) {
		t.Fatal("expression cleanup retry changed its receipt")
	}
	receipt = nil
}
