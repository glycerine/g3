package pdr

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPDRModelValidationPreservesPartialOwnersWhenCleanupRefuses(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var owner *pdrValidationAttempt
	originalCause := errors.New("PDR model export failed")
	var failure memory.ConstructionFailure
	active, callback, intact := false, false, true
	var runtime *memory.Runtime
	var err error
	runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			modelOwner := owner.modelOwners[0].model
			abstractorOwner := owner.abstractors[0]
			referenceOwner := owner.references[0]
			cause := finishPDRValidation(owner, originalCause)
			intact = errors.Is(cause, originalCause) && errors.Is(cause, ast.ErrASTPending) && errors.As(cause, &failure) && owner.modelOwners[0].model == modelOwner && len(owner.abstractors) == 1 && owner.abstractors[0] == abstractorOwner && len(owner.references) == 1 && owner.references[0] == referenceOwner && failure.RuntimeOwner() == runtime
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
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if failure != nil {
			must(failure.Cleanup())
		}
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("PDR model export fixture leaked bytes", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	constantName, err := symbols.Intern("pending-pdr-model-constant")
	must(err)
	constant, err := manager.MkFuncDecl(constantName, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	functionName, err := symbols.Intern("pending-pdr-model-function")
	must(err)
	function, err := manager.MkFuncDecl(functionName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	_ = function
	partial, err := model.NewModel(manager)
	must(err)
	must(partial.Core().RegisterConstant(constant, truth))
	interpretation, err := model.NewFunctionInterpretation(manager, 1)
	must(err)
	must(interpretation.SetElse(truth))
	reference, err := manager.NewRef(truth)
	must(err)
	abstractor, err := ast.NewExprAbstractor(manager)
	must(err)
	owner = newPDRValidationAttempt(manager)
	owner.abstractors = append(owner.abstractors, abstractor)
	owner.modelOwners = append(owner.modelOwners, &pdrModelExportOwner{manager: manager, model: partial, interpretations: []*model.FunctionInterpretation{interpretation}})
	owner.references = append(owner.references, reference)
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
	if body == nil || !callback || !intact || failure == nil {
		t.Fatal("native refusal did not preserve model export owners", body, callback, intact)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(failure.Cleanup())
	must(failure.Cleanup())
	if failure.CleanupCause() != nil || len(owner.modelOwners) != 0 || len(owner.abstractors) != 0 || len(owner.references) != 0 {
		t.Fatal("model export cleanup retry did not finish")
	}
}
