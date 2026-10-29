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
	uparams "github.com/glycerine/g3/pkg/util/params"
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
	context, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget})
	quantifierResultMust(t, err)
	t.Cleanup(func() { quantifierResultMust(t, context.Close()) })
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	quantifierResultMust(t, err)
	t.Cleanup(func() { quantifierResultMust(t, catalog.Close()) })
	globals, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	quantifierResultMust(t, err)
	quantifierResultMust(t, globals.InitializeSource())
	t.Cleanup(func() { quantifierResultMust(t, globals.Close()) })
	engine, err := NewPropositionalEngineWithServices(manager, context, globals)
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
	functionalZero, err := manager.ArithmeticIntegerNumeralText("0")
	quantifierResultMust(t, err)
	functionalInteger, err := manager.GetSort(functionalZero)
	quantifierResultMust(t, err)
	functionalX, err := manager.MkVar(0, functionalInteger)
	quantifierResultMust(t, err)
	inlinePredicate, err = manager.MkFuncDecl(inlineName, []ast.ASTHandle{functionalInteger, functionalInteger}, boolean, ast.ASTFuncDeclInfo{})
	quantifierResultMust(t, err)
	functionalHead, err := manager.MkApp(inlinePredicate, []ast.ASTHandle{functionalX, functionalZero})
	quantifierResultMust(t, err)
	functionalHeadRef, err := manager.NewRef(functionalHead)
	quantifierResultMust(t, err)
	functionalBodyRef, err := manager.NewRef(truth)
	quantifierResultMust(t, err)
	functionalRule := transforms.InlineModelRule{Head: functionalHeadRef, Body: functionalBodyRef}
	functionalSource, err := model.NewModel(manager)
	quantifierResultMust(t, err)
	functionalExpanded, err := engine.expandHornModel(functionalSource, []hornModelConversion{{kind: hornConvertInline, inline: []transforms.InlineModelRule{functionalRule}}})
	quantifierResultMust(t, err)
	functionalInterpretation, err := functionalExpanded.Core().FunctionInterpretation(inlinePredicate)
	quantifierResultMust(t, err)
	functionalFormula, err := functionalInterpretation.Else()
	quantifierResultMust(t, err)
	functionalUsed, err := ast.NewASTUsedVars(manager)
	quantifierResultMust(t, err)
	quantifierResultMust(t, functionalUsed.Process(functionalFormula))
	if functionalUsed.Contains(1) == (ast.ASTHandle{}) {
		t.Fatal("inline functional clause did not retain its constrained argument", functionalFormula)
	}
	quantifierResultMust(t, functionalExpanded.Close())
	quantifierResultMust(t, manager.DelRef(&functionalBodyRef))
	quantifierResultMust(t, manager.DelRef(&functionalHeadRef))

	missingName, err := manager.Symbols.Intern("horn-inline-missing")
	quantifierResultMust(t, err)
	missing, err := manager.MkFuncDecl(missingName, nil, boolean, ast.ASTFuncDeclInfo{})
	quantifierResultMust(t, err)
	missingApp, err := manager.MkApp(missing, nil)
	quantifierResultMust(t, err)
	resultName, err := manager.Symbols.Intern("horn-inline-result")
	quantifierResultMust(t, err)
	resultDeclaration, err := manager.MkFuncDecl(resultName, nil, boolean, ast.ASTFuncDeclInfo{})
	quantifierResultMust(t, err)
	resultApp, err := manager.MkApp(resultDeclaration, nil)
	quantifierResultMust(t, err)
	makeRule := func(body ast.ASTHandle) transforms.InlineModelRule {
		headRef, makeErr := manager.NewRef(resultApp)
		quantifierResultMust(t, makeErr)
		bodyRef, makeErr := manager.NewRef(body)
		quantifierResultMust(t, makeErr)
		return transforms.InlineModelRule{Head: headRef, Body: bodyRef}
	}
	inlineRules := []transforms.InlineModelRule{makeRule(truth), makeRule(missingApp)}
	t.Cleanup(func() {
		for index := len(inlineRules) - 1; index >= 0; index-- {
			quantifierResultMust(t, manager.DelRef(&inlineRules[index].Body))
			quantifierResultMust(t, manager.DelRef(&inlineRules[index].Head))
		}
	})
	inlineSource, err := model.NewModel(manager)
	quantifierResultMust(t, err)
	inlineExpanded, err := engine.expandHornModel(inlineSource, []hornModelConversion{{kind: hornConvertInline, inline: inlineRules}})
	quantifierResultMust(t, err)
	t.Cleanup(func() { quantifierResultMust(t, inlineExpanded.Close()) })
	value, err = inlineExpanded.Core().ConstantInterpretation(missing)
	quantifierResultMust(t, err)
	if !manager.IsFalse(value) {
		t.Fatal("missing inline body predicate did not default false", value)
	}
	value, err = inlineExpanded.Core().ConstantInterpretation(resultDeclaration)
	quantifierResultMust(t, err)
	if !manager.IsTrue(value) {
		t.Fatal("inline clauses did not merge by disjunction", value)
	}
}
