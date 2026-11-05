package fp

// Source: src/muz/transforms/dl_mk_scale.cpp scale_model_converter
// substitution of the homogeneous coordinate with one.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerScaleReconstructsOriginalPredicateModel(t *testing.T) {
	budget := arena.NewBudget(1 << 26)
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
	one, err := manager.ArithmeticRealNumeralText("1")
	quantifierResultMust(t, err)
	realSort, err := manager.GetSort(one)
	quantifierResultMust(t, err)
	oldName, err := manager.Symbols.Intern("scale-model-old")
	quantifierResultMust(t, err)
	old, err := manager.MkFuncDecl(oldName, []ast.ASTHandle{realSort}, boolean, ast.ASTFuncDeclInfo{})
	quantifierResultMust(t, err)
	newName, err := manager.Symbols.Intern("scale-model-new")
	quantifierResultMust(t, err)
	derived, err := manager.MkFuncDecl(newName, []ast.ASTHandle{realSort, realSort}, boolean, ast.ASTFuncDeclInfo{})
	quantifierResultMust(t, err)
	oldRef, err := manager.NewRef(old)
	quantifierResultMust(t, err)
	derivedRef, err := manager.NewRef(derived)
	quantifierResultMust(t, err)
	t.Cleanup(func() {
		quantifierResultMust(t, manager.DelRef(&derivedRef))
		quantifierResultMust(t, manager.DelRef(&oldRef))
	})
	source, err := model.NewModel(manager)
	quantifierResultMust(t, err)
	interpretation, err := model.NewFunctionInterpretation(manager, 2)
	quantifierResultMust(t, err)
	value, err := manager.MkVar(0, realSort)
	quantifierResultMust(t, err)
	sigma, err := manager.MkVar(1, realSort)
	quantifierResultMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	quantifierResultMust(t, err)
	body, err := manager.MkFamilyApp(arithmetic, 5, nil, []ast.ASTHandle{value, sigma})
	quantifierResultMust(t, err)
	quantifierResultMust(t, interpretation.SetElse(body))
	quantifierResultMust(t, source.Core().RegisterFunction(derived, interpretation))
	engine.spacerScaled = map[ast.ASTHandle]ast.ASTHandle{old: derived}
	expanded, err := engine.expandSpacerModel(source)
	quantifierResultMust(t, err)
	t.Cleanup(func() { quantifierResultMust(t, expanded.Close()) })
	reconstructed, err := expanded.Core().FunctionInterpretation(old)
	quantifierResultMust(t, err)
	if reconstructed == nil || reconstructed.Arity() != 1 {
		t.Fatal("original scaled interpretation was not reconstructed")
	}
	got, err := reconstructed.Else()
	quantifierResultMust(t, err)
	argument, err := manager.MkVar(0, realSort)
	quantifierResultMust(t, err)
	want, err := manager.MkFamilyApp(arithmetic, 5, nil, []ast.ASTHandle{argument, one})
	quantifierResultMust(t, err)
	if got != want {
		t.Fatalf("reconstructed body %v, want %v", got, want)
	}

	// A nullary original becomes a unary scaled predicate. The current C++
	// converter substitutes sigma = 1 and registers the reconstructed value
	// as a constant, matching the model's nullary lookup path.
	nullaryName, err := manager.Symbols.Intern("scale-model-nullary")
	quantifierResultMust(t, err)
	nullary, err := manager.MkFuncDecl(nullaryName, nil, boolean, ast.ASTFuncDeclInfo{})
	quantifierResultMust(t, err)
	nullaryRef, err := manager.NewRef(nullary)
	quantifierResultMust(t, err)
	t.Cleanup(func() { quantifierResultMust(t, manager.DelRef(&nullaryRef)) })
	scaledName, err := manager.Symbols.Intern("scale-model-nullary-scaled")
	quantifierResultMust(t, err)
	scaled, err := manager.MkFuncDecl(scaledName, []ast.ASTHandle{realSort}, boolean, ast.ASTFuncDeclInfo{})
	quantifierResultMust(t, err)
	scaledRef, err := manager.NewRef(scaled)
	quantifierResultMust(t, err)
	t.Cleanup(func() { quantifierResultMust(t, manager.DelRef(&scaledRef)) })
	nullaryModel, err := model.NewModel(manager)
	quantifierResultMust(t, err)
	nullaryInterpretation, err := model.NewFunctionInterpretation(manager, 1)
	quantifierResultMust(t, err)
	scaleVariable, err := manager.MkVar(0, realSort)
	quantifierResultMust(t, err)
	nullaryBody, err := manager.MkFamilyApp(arithmetic, 5, nil, []ast.ASTHandle{scaleVariable, zeroForScaleTest(t, manager)})
	quantifierResultMust(t, err)
	quantifierResultMust(t, nullaryInterpretation.SetElse(nullaryBody))
	quantifierResultMust(t, nullaryModel.Core().RegisterFunction(scaled, nullaryInterpretation))
	engine.spacerScaled = map[ast.ASTHandle]ast.ASTHandle{nullary: scaled}
	nullaryExpanded, err := engine.expandSpacerModel(nullaryModel)
	quantifierResultMust(t, err)
	t.Cleanup(func() { quantifierResultMust(t, nullaryExpanded.Close()) })
	nullaryValue, err := nullaryExpanded.Core().ConstantInterpretation(nullary)
	quantifierResultMust(t, err)
	wantNullary, err := manager.MkFamilyApp(arithmetic, 5, nil, []ast.ASTHandle{one, zeroForScaleTest(t, manager)})
	quantifierResultMust(t, err)
	if nullaryValue != wantNullary {
		t.Fatalf("nullary reconstructed value %v, want %v", nullaryValue, wantNullary)
	}
	if present, err := nullaryExpanded.Core().HasInterpretation(scaled); err != nil || present {
		t.Fatalf("scaled predicate remains in model: %v, %v", present, err)
	}
}

func zeroForScaleTest(t *testing.T, manager *ast.ASTManager) ast.ASTHandle {
	t.Helper()
	zero, err := manager.ArithmeticRealNumeralText("0")
	quantifierResultMust(t, err)
	return zero
}
