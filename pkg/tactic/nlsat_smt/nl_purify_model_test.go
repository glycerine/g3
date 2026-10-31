package nlsatsmt

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestNLPurifyMergeModelsRewritesRealFunctionGraphs(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	x := analysisConst(t, m, real, "merge-x")
	xdecl, err := m.GetDecl(x)
	astTestMust(t, err)
	h, err := m.ArithmeticRealNumeralText("1")
	one := analysisKeep(t, m, h, err)
	h, err = m.ArithmeticRealNumeralText("2")
	two := analysisKeep(t, m, h, err)
	h, err = m.ArithmeticIntegerNumeralText("1")
	intOne := analysisKeep(t, m, h, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "merge-f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	g, err := m.MkFuncDecl(astTestSymbol(t, m, "merge-g"), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	nl, err := model.NewModel(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, nl.Close()) }()
	smt, err := model.NewModel(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, smt.Close()) }()
	astTestMust(t, nl.Core().RegisterConstant(xdecl, two))
	astTestMust(t, smt.Core().RegisterConstant(xdecl, one))
	fgraph, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, fgraph.InsertEntry([]ast.ASTHandle{one}, one))
	astTestMust(t, fgraph.SetElse(one))
	astTestMust(t, smt.Core().RegisterFunction(f, fgraph))
	ggraph, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, ggraph.InsertEntry([]ast.ASTHandle{intOne}, intOne))
	astTestMust(t, ggraph.SetElse(intOne))
	astTestMust(t, smt.Core().RegisterFunction(g, ggraph))
	astTestMust(t, mergeNLModels(m, nl.Core(), smt.Core()))
	value, err := smt.Core().ConstantInterpretation(xdecl)
	astTestMust(t, err)
	if value != two {
		t.Fatal("constant", value, two)
	}
	merged, err := smt.Core().FunctionInterpretation(f)
	astTestMust(t, err)
	entry, err := merged.Entry(0)
	astTestMust(t, err)
	arg, err := entry.Argument(0)
	astTestMust(t, err)
	result, err := entry.Result()
	astTestMust(t, err)
	fallback, err := merged.Else()
	astTestMust(t, err)
	if arg != two || result != two || fallback != two {
		t.Fatal("translated graph", arg, result, fallback)
	}
	untouched, err := smt.Core().FunctionInterpretation(g)
	astTestMust(t, err)
	if untouched != ggraph {
		t.Fatal("integer-only graph was rewritten")
	}
}
