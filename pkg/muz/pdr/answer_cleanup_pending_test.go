package pdr

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPDRAnswerExportPreservesResultWhenConstraintCleanupRefuses(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var owner *pdrAnswerExportOwner
	var published ast.ASTRef
	var publicationErr error
	var failure memory.ConstructionFailure
	active, callback, intact := false, false, true
	var runtime *memory.Runtime
	var err error
	runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			resultRoot := published
			temporaryRoot := owner.temporaries[0]
			owner.finish(&published, &publicationErr)
			intact = published == (ast.ASTRef{}) && errors.Is(publicationErr, ast.ErrASTPending) && errors.As(publicationErr, &failure) && owner.result == resultRoot && len(owner.temporaries) == 1 && owner.temporaries[0] == temporaryRoot && failure.RuntimeOwner() == runtime
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
			t.Fatal("PDR answer export fixture leaked bytes", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("pending-pdr-answer")
	must(err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	application, err := manager.MkApp(declaration, nil)
	must(err)
	temporary, err := manager.NewRef(application)
	must(err)
	published, err = manager.NewRef(application)
	must(err)
	owner = &pdrAnswerExportOwner{manager: manager, temporaries: []ast.ASTRef{temporary}}
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
		t.Fatal("native refusal did not preserve answer export owners", body, callback, intact, publicationErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(failure.Cleanup())
	must(failure.Cleanup())
	if failure.CleanupCause() != nil || owner.result != (ast.ASTRef{}) || len(owner.temporaries) != 0 {
		t.Fatal("answer export cleanup retry did not finish")
	}
	if manager.IsLiveHandle(application) {
		t.Fatal("answer export roots remained live")
	}
}
