package fp

// Source: src/tactic/model_converter.cpp concatenation order and
// src/tactic/extension_model_converter.cpp reverse entry replay.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/transforms"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornModelConversionsReplayInReverseApplicationOrder(t *testing.T) {
	budget := arena.NewBudget(1 << 24)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	quantifierResultMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	quantifierResultMust(t, err)
	t.Cleanup(func() {
		quantifierResultMust(t, numerals.Close())
		quantifierResultMust(t, symbols.Close())
		if used := budget.Used(); used != 0 {
			t.Errorf("owner graph retained %d bytes", used)
		}
	})
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	quantifierResultMust(t, err)
	quantifierResultMust(t, manager.RegisterDeclarationPlugins())
	t.Cleanup(func() { quantifierResultMust(t, manager.Close()) })
	engine, err := NewPropositionalEngine(manager)
	quantifierResultMust(t, err)
	t.Cleanup(func() { quantifierResultMust(t, engine.Close()) })

	boolean, err := manager.MkBoolSort()
	quantifierResultMust(t, err)
	name, err := manager.Symbols.Intern("horn-conversion-order")
	quantifierResultMust(t, err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	quantifierResultMust(t, err)
	predicateRef, err := manager.NewRef(predicate)
	quantifierResultMust(t, err)
	truth, err := manager.MkTrue()
	quantifierResultMust(t, err)
	truthRef, err := manager.NewRef(truth)
	quantifierResultMust(t, err)
	falsity, err := manager.MkFalse()
	quantifierResultMust(t, err)
	falsityRef, err := manager.NewRef(falsity)
	quantifierResultMust(t, err)
	t.Cleanup(func() {
		quantifierResultMust(t, manager.DelRef(&falsityRef))
		quantifierResultMust(t, manager.DelRef(&truthRef))
		quantifierResultMust(t, manager.DelRef(&predicateRef))
	})
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	quantifierResultMust(t, err)
	integer, err := manager.GetSort(zero)
	quantifierResultMust(t, err)
	inlineName, err := manager.Symbols.Intern("horn-inline-normalize")
	quantifierResultMust(t, err)
	inlinePredicate, err := manager.MkFuncDecl(inlineName, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	quantifierResultMust(t, err)
	x, err := manager.MkVar(0, integer)
	quantifierResultMust(t, err)
	y, err := manager.MkVar(1, integer)
	quantifierResultMust(t, err)
	inlineHead, err := manager.MkApp(inlinePredicate, []ast.ASTHandle{x, x})
	quantifierResultMust(t, err)
	inlineBody, err := manager.MkEq(y, zero)
	quantifierResultMust(t, err)
	inlineHeadRef, err := manager.NewRef(inlineHead)
	quantifierResultMust(t, err)
	inlineBodyRef, err := manager.NewRef(inlineBody)
	quantifierResultMust(t, err)
	declaration, normalized, err := engine.normalizeInlineModelRule(transforms.InlineModelRule{Head: inlineHeadRef, Body: inlineBodyRef})
	quantifierResultMust(t, err)
	if declaration != inlinePredicate {
		t.Fatal("normalized inline predicate", declaration)
	}
	normalizedHandle, err := manager.RefGet(normalized)
	quantifierResultMust(t, err)
	forall, err := manager.IsForall(normalizedHandle)
	quantifierResultMust(t, err)
	declarations, err := manager.GetNumDeclarations(normalizedHandle)
	quantifierResultMust(t, err)
	if forall || declarations != 2 {
		t.Fatal("normalized inline existential", forall, declarations)
	}
	quantifierResultMust(t, manager.DelRef(&normalized))
	quantifierResultMust(t, manager.DelRef(&inlineBodyRef))
	quantifierResultMust(t, manager.DelRef(&inlineHeadRef))

	source, err := model.NewModel(manager)
	quantifierResultMust(t, err)
	conversions := []hornModelConversion{
		{kind: hornConvertCone, cone: transforms.ConeModelStep{{Declaration: predicate, Formula: truthRef}}},
		{kind: hornConvertCone, cone: transforms.ConeModelStep{{Declaration: predicate, Formula: falsityRef}}},
	}
	expanded, err := engine.expandHornModel(source, conversions)
	quantifierResultMust(t, err)
	t.Cleanup(func() { quantifierResultMust(t, expanded.Close()) })
	value, err := expanded.Core().ConstantInterpretation(predicate)
	quantifierResultMust(t, err)
	if !manager.IsTrue(value) {
		t.Fatal("Horn converters did not replay newest-to-oldest", value)
	}
}
