package api

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestAPICurrentModelRetainedAcrossLaterChecks(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	script := `(set-logic QF_UF)
(declare-const retained Bool)
(assert retained)
(check-sat)`
	astTestMust(t, context.ExecuteSMT2(strings.NewReader(script), io.Discard))
	assertions, err := context.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("assertion count", len(assertions))
	}
	manager, err := context.Manager()
	astTestMust(t, err)
	declaration, err := manager.GetDecl(assertions[0])
	astTestMust(t, err)
	old, err := context.CurrentAPIModel()
	astTestMust(t, err)
	before, err := old.ConstantInterpretation(declaration)
	astTestMust(t, err)
	if before.IsNull() {
		t.Fatal("retained model lacks asserted constant")
	}

	later := `(push)
(assert (not retained))
(check-sat)
(pop)
(check-sat)`
	astTestMust(t, context.ExecuteSMT2(strings.NewReader(later), io.Discard))
	after, err := old.ConstantInterpretation(declaration)
	astTestMust(t, err)
	if after != before {
		t.Fatal("later checks mutated retained model", before, after)
	}
	astTestMust(t, old.Close())
	astTestMust(t, context.Close())
}

func TestAPIModelLifecycleEnumerationMutationAndFunctionWrappers(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	manager, err := context.Manager()
	astTestMust(t, err)
	modelObject, err := context.NewAPIModel()
	astTestMust(t, err)
	boolSort, err := manager.MkBoolSort()
	astTestMust(t, err)
	yes, err := manager.MkTrue()
	astTestMust(t, err)
	no, err := manager.MkFalse()
	astTestMust(t, err)
	constant, err := manager.MkFuncDecl(astTestSymbol(t, manager, "api-model-constant"), nil, boolSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestMust(t, modelObject.RegisterConstant(constant, yes))
	if n, e := modelObject.NumConstants(); e != nil || n != 1 {
		t.Fatal("constant count", n, e)
	}
	if declaration, e := modelObject.ConstantDeclaration(0); e != nil || declaration != constant {
		t.Fatal("constant declaration", declaration, e)
	}
	if value, e := modelObject.ConstantInterpretation(constant); e != nil || value != yes {
		t.Fatal("constant interpretation", value, e)
	}
	if found, e := modelObject.HasInterpretation(constant); e != nil || !found {
		t.Fatal("constant presence", found, e)
	}
	if display, e := modelObject.String(); e != nil || !strings.Contains(display, "api-model-constant") {
		t.Fatal("model display", display, e)
	}

	functionDecl, err := manager.MkFuncDecl(astTestSymbol(t, manager, "api-model-function"), []ast.ASTHandle{boolSort}, boolSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	function, err := modelObject.AddFunctionInterpretation(functionDecl, no)
	astTestMust(t, err)
	astTestMust(t, function.AddEntry([]ast.ASTHandle{no}, yes))
	if arity, e := function.Arity(); e != nil || arity != 1 {
		t.Fatal("function arity", arity, e)
	}
	if fallback, e := function.Else(); e != nil || fallback != no {
		t.Fatal("function else", fallback, e)
	}
	if n, e := function.NumEntries(); e != nil || n != 1 {
		t.Fatal("function entries", n, e)
	}
	entry, err := function.Entry(0)
	astTestMust(t, err)
	if n, e := entry.NumArguments(); e != nil || n != 1 {
		t.Fatal("entry arity", n, e)
	}
	if argument, e := entry.Argument(0); e != nil || argument != no {
		t.Fatal("entry argument", argument, e)
	}
	if value, e := entry.Value(); e != nil || value != yes {
		t.Fatal("entry value", value, e)
	}
	if _, e := entry.Argument(1); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("entry bounds", e)
	}
	if n, e := modelObject.NumFunctions(); e != nil || n != 1 {
		t.Fatal("function count", n, e)
	}
	if declaration, e := modelObject.FunctionDeclaration(0); e != nil || declaration != functionDecl {
		t.Fatal("function declaration", declaration, e)
	}
	borrowed, err := modelObject.FunctionInterpretation(functionDecl)
	astTestMust(t, err)

	// All three wrappers retain the model after its primary API handle closes.
	astTestMust(t, modelObject.Close())
	if value, e := entry.Value(); e != nil || value != yes {
		t.Fatal("entry lost retained model", value, e)
	}
	astTestMust(t, borrowed.Close())
	astTestMust(t, function.Close())
	astTestMust(t, entry.Close())
	astTestMust(t, context.Close())
}

func TestAPIModelUniverseOwnsValuesAndTranslation(t *testing.T) {
	sourceContext, err := NewSMTContext()
	astTestMust(t, err)
	targetContext, err := NewSMTContext()
	astTestMust(t, err)
	sourceManager, err := sourceContext.Manager()
	astTestMust(t, err)
	modelObject, err := sourceContext.NewAPIModel()
	astTestMust(t, err)
	sort, err := sourceManager.MkUninterpretedSort(astTestSymbol(t, sourceManager, "api-model-sort"), nil)
	astTestMust(t, err)
	value, err := modelObject.core.DerivedModel().GetSomeValue(sort)
	astTestMust(t, err)
	astTestMust(t, modelObject.RegisterUniverse(sort, []ast.ASTHandle{value}))
	universe, err := modelObject.Universe(sort)
	astTestMust(t, err)
	if universe.Size() != 1 {
		t.Fatal("universe size", universe.Size())
	}
	owned, err := universe.At(0)
	astTestMust(t, err)
	if owned != value {
		t.Fatal("universe value", owned, value)
	}
	translated, err := modelObject.Translate(targetContext)
	astTestMust(t, err)
	if n, e := translated.NumSorts(); e != nil || n != 1 {
		t.Fatal("translated sorts", n, e)
	}
	translatedSort, err := translated.Sort(0)
	astTestMust(t, err)
	translatedUniverse, err := translated.Universe(translatedSort)
	astTestMust(t, err)
	if translatedUniverse.Size() != 1 {
		t.Fatal("translated universe", translatedUniverse.Size())
	}

	astTestMust(t, modelObject.Close())
	if _, err = sourceManager.GetKind(owned); err != nil {
		t.Fatal("universe vector did not retain value", err)
	}
	astTestMust(t, universe.Close())
	astTestMust(t, translatedUniverse.Close())
	astTestMust(t, translated.Close())
	astTestMust(t, sourceContext.Close())
	astTestMust(t, targetContext.Close())
}

func TestAPIModelAsArrayQueries(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	manager, err := context.Manager()
	astTestMust(t, err)
	boolSort, err := manager.MkBoolSort()
	astTestMust(t, err)
	function, err := manager.MkFuncDecl(astTestSymbol(t, manager, "api-as-array"), []ast.ASTHandle{boolSort}, boolSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	array, err := context.AsArray(function)
	astTestMust(t, err)
	if yes, e := context.IsAsArray(array); e != nil || !yes {
		t.Fatal("as-array recognition", yes, e)
	}
	if declaration, e := context.AsArrayFunction(array); e != nil || declaration != function {
		t.Fatal("as-array declaration", declaration, e)
	}
	plain, err := manager.MkTrue()
	astTestMust(t, err)
	if yes, e := context.IsAsArray(plain); e != nil || yes {
		t.Fatal("plain expression recognized as array", yes, e)
	}
	astTestMust(t, context.Close())
}
