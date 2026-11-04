package pdr

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPDRCoverExportPreservesUnpublishedResultWhenCleanupRefuses(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var owner *pdrCoverExportOwner
	originalCause := errors.New("PDR cover export failed")
	var failure memory.ConstructionFailure
	active, callback, intact := false, false, true
	var runtime *memory.Runtime
	var err error
	runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			published := owner.result
			owner.result = ast.ASTRef{}
			publicationErr := originalCause
			owner.finish(&published, &publicationErr)
			intact = published == (ast.ASTRef{}) && errors.Is(publicationErr, originalCause) && errors.Is(publicationErr, ast.ErrASTPending) && errors.As(publicationErr, &failure) && owner.hasSubstitution && owner.formula != (ast.ASTRef{}) && owner.result != (ast.ASTRef{}) && failure.RuntimeOwner() == runtime
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
			t.Fatal("PDR cover export fixture leaked bytes", budget.Used())
		}
	}()
	truth, err := manager.MkTrue()
	must(err)
	substitution, err := manager.NewExprSubstitution()
	must(err)
	must(manager.ExprSubstitutionInsert(substitution, truth, truth, ast.ASTHandle{}, ast.ASTDependency{}))
	replacer, err := rewriter.NewDefaultExprReplacer(manager)
	must(err)
	must(replacer.SetSubstitution(&substitution))
	result, err := manager.NewRef(truth)
	must(err)
	formula, err := manager.NewRef(truth)
	must(err)
	owner = &pdrCoverExportOwner{manager: manager, substitution: substitution, hasSubstitution: true, replacer: replacer, formula: formula, result: result}
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
		t.Fatal("native refusal did not preserve cover export owners", body, callback, intact)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(failure.Cleanup())
	must(failure.Cleanup())
	if failure.CleanupCause() != nil || owner.replacer != nil || owner.hasSubstitution || owner.formula != (ast.ASTRef{}) || owner.result != (ast.ASTRef{}) {
		t.Fatal("cover export cleanup retry did not finish")
	}
}
