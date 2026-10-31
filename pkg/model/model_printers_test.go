package model_test

// Source: src/model/model_pp.cpp; src/model/model_smt2_pp.cpp.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestModelPrintersReleaseTemporaryFormatTrees(t *testing.T) {
	m, md, _, context, _, config := evaluatorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	value, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	constant, err := m.MkConst(astTestSymbol(t, m, "owned-printer-document"), sort)
	astTestMust(t, err)
	astTestPin(t, m, constant)
	decl, err := m.GetDecl(constant)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterConstant(decl, value))
	// Initialize the persistent format-manager declarations before measuring
	// lexical documents. Distinct numeral values below force distinct documents.
	var warm bytes.Buffer
	astTestMust(t, md.Core().WriteSMT2(&warm, context, config.Globals, 0))
	astTestMust(t, md.Core().WriteDebug(&warm, context, config.Globals))
	formats, err := m.GetFormatManager()
	astTestMust(t, err)
	before := formats.Stats()
	for i := 1; i <= 3; i++ {
		value, err = m.ArithmeticIntegerNumeralText(fmt.Sprint(i))
		astTestMust(t, err)
		astTestMust(t, md.Core().RegisterConstant(decl, value))
		var output bytes.Buffer
		astTestMust(t, md.Core().WriteSMT2(&output, context, config.Globals, 0))
		astTestMust(t, md.Core().WriteDebug(&output, context, config.Globals))
		after := formats.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
			t.Fatal("model printer kept temporary format trees", before, after)
		}
		if !strings.Contains(output.String(), "owned-printer-document") {
			t.Fatal("printer output missing after document cleanup")
		}
	}
}

func TestModelDebugPrinterSourceOrderAndFunctionGraph(t *testing.T) {
	m, md, _, context, _, config := evaluatorFixture(t)
	no, err := m.MkFalse()
	astTestMust(t, err)
	yes, err := m.MkTrue()
	astTestMust(t, err)
	constant := goalTestAtom(t, m, "debug-constant")
	decl, err := m.GetDecl(constant)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterConstant(decl, no))
	f := modelTestFunctionDecl(t, m, "debug-function")
	fi := modelTestFunction(t, m, 1)
	astTestMust(t, fi.InsertEntry([]ast.ASTHandle{no}, yes))
	variable, err := m.MkVar(0, mustBoolSort(t, m))
	astTestMust(t, err)
	astTestMust(t, fi.SetElse(variable))
	astTestMust(t, md.Core().RegisterFunction(f, fi))
	var out bytes.Buffer
	astTestMust(t, md.Core().WriteDebug(&out, context, config.Globals))
	want := "(define debug-constant false)\n" +
		"(define (debug-function x!0)\n" +
		"  (if (= x!0 false) true\n" +
		"    x!0))\n"
	if out.String() != want {
		t.Fatalf("model_pp output:\n got %q\nwant %q", out.String(), want)
	}
}

func TestModelSMT2PrinterUniversesQuotedNamesAndDependencyOrder(t *testing.T) {
	m, md, _, context, _, config := evaluatorFixture(t)
	boolSort := mustBoolSort(t, m)
	no, err := m.MkFalse()
	astTestMust(t, err)
	yes, err := m.MkTrue()
	astTestMust(t, err)

	u, err := m.MkUninterpretedSort(astTestSymbol(t, m, "printer-universe"), nil)
	astTestMust(t, err)
	u0, err := m.GetSomeValue(u)
	astTestMust(t, err)
	astTestMust(t, md.RegisterUniverse(u, []ast.ASTHandle{u0}))

	constant, err := m.MkConst(astTestSymbol(t, m, "quoted constant"), boolSort)
	astTestMust(t, err)
	constantDecl, err := m.GetDecl(constant)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterConstant(constantDecl, no))

	f, err := m.MkFuncDecl(astTestSymbol(t, m, "printer-f"), []ast.ASTHandle{boolSort}, boolSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	g, err := m.MkFuncDecl(astTestSymbol(t, m, "printer-g"), []ast.ASTHandle{boolSort}, boolSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolSort)
	astTestMust(t, err)
	fApp, err := m.MkApp(f, []ast.ASTHandle{variable})
	astTestMust(t, err)
	gInterp, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, gInterp.InsertEntry([]ast.ASTHandle{no}, yes))
	astTestMust(t, gInterp.SetElse(fApp))
	// Register the dependent first: model_smt2_pp must still print f first.
	astTestMust(t, md.Core().RegisterFunction(g, gInterp))
	fInterp, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, fInterp.SetElse(yes))
	astTestMust(t, md.Core().RegisterFunction(f, fInterp))

	var out bytes.Buffer
	astTestMust(t, md.Core().WriteSMT2(&out, context, config.Globals, 2))
	text := out.String()
	checks := []string{
		"  ;; universe for printer-universe:\n",
		"  ;; definitions for universe elements:\n",
		"  ;; cardinality constraint:\n",
		"  (define-fun |quoted constant| () Bool\n    false)\n",
		"  (define-fun printer-f ((x!0 Bool)) Bool\n    true)\n",
		"  (define-fun printer-g ((x!0 Bool)) Bool\n    (ite (= x!0 false) true (printer-f x!0)))\n",
	}
	for _, check := range checks {
		if !strings.Contains(text, check) {
			t.Fatalf("model_smt2_pp output missing %q:\n%s", check, text)
		}
	}
	if strings.Index(text, "define-fun printer-f") > strings.Index(text, "define-fun printer-g") {
		t.Fatalf("dependency was emitted after dependent:\n%s", text)
	}
}
