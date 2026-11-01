package model_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestModelImplicantNormalizesFunctionArraysInsideStoreRows(t *testing.T) {
	for _, position := range []string{"index", "value", "deep-value"} {
		t.Run(position, func(t *testing.T) {
			m, md, implicant := modelImplicantFixture(t)
			family, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			boolean := mustBoolSort(t, m)
			yes, err := m.MkTrue()
			astTestMust(t, err)
			no, err := m.MkFalse()
			astTestMust(t, err)
			function, err := m.MkFuncDecl(astTestSymbol(t, m, "nested-array-function"), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			astTestPin(t, m, function)
			graph, err := model.NewFunctionInterpretation(m, 1)
			astTestMust(t, err)
			astTestMust(t, graph.SetElse(no))
			astTestMust(t, graph.InsertEntry([]ast.ASTHandle{yes}, yes))
			astTestMust(t, md.Core().RegisterFunction(function, graph))
			inner, err := m.MkAsArray(function)
			astTestMust(t, err)
			astTestPin(t, m, inner)
			innerSort, err := m.GetSort(inner)
			astTestMust(t, err)
			innerBase, err := m.MkConstArray(innerSort, no)
			astTestMust(t, err)
			astTestPin(t, m, innerBase)
			normalized, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{innerBase, yes, yes})
			astTestMust(t, err)
			astTestPin(t, m, normalized)
			if position == "deep-value" {
				function2, err := m.MkFuncDecl(astTestSymbol(t, m, "nested-array-function2"), []ast.ASTHandle{boolean}, innerSort, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				astTestPin(t, m, function2)
				graph2, err := model.NewFunctionInterpretation(m, 1)
				astTestMust(t, err)
				astTestMust(t, graph2.SetElse(innerBase))
				astTestMust(t, graph2.InsertEntry([]ast.ASTHandle{yes}, inner))
				astTestMust(t, md.Core().RegisterFunction(function2, graph2))
				inner, err = m.MkAsArray(function2)
				astTestMust(t, err)
				astTestPin(t, m, inner)
				innerSort, err = m.GetSort(inner)
				astTestMust(t, err)
				innerBase, err = m.MkConstArray(innerSort, innerBase)
				astTestMust(t, err)
				astTestPin(t, m, innerBase)
				normalized, err = m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{innerBase, yes, normalized})
				astTestMust(t, err)
				astTestPin(t, m, normalized)
			}
			domain, rangeSort := boolean, innerSort
			fallback, index, value := innerBase, yes, inner
			expectedIndex, expectedValue := yes, normalized
			if position == "index" {
				domain, rangeSort = innerSort, boolean
				fallback, index, value = no, inner, yes
				expectedIndex, expectedValue = normalized, yes
			}
			outerSort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{domain}), rangeSort)
			astTestMust(t, err)
			base, err := m.MkConstArray(outerSort, fallback)
			astTestMust(t, err)
			astTestPin(t, m, base)
			input, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, index, value})
			astTestMust(t, err)
			astTestPin(t, m, input)
			variable, err := m.MkFreshConst("nested-array-model-value", outerSort)
			astTestMust(t, err)
			astTestPin(t, m, variable)
			decl, err := m.GetDecl(variable)
			astTestMust(t, err)
			astTestMust(t, md.Core().RegisterConstant(decl, input))
			expected, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, expectedIndex, expectedValue})
			astTestMust(t, err)
			astTestPin(t, m, expected)
			before := m.Stats().LiveReferences
			result, err := implicant.EvalExpr(md, variable)
			astTestMust(t, err)
			actual, err := m.RefGet(result)
			astTestMust(t, err)
			if actual != expected {
				t.Fatal("nested function-backed array was not normalized", position, actual, expected)
			}
			astTestMust(t, m.DelRef(&result))
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("nested normalization leaked reference slots", got, before)
			}
		})
	}
}
