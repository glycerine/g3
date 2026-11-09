package model_test

// Source: src/model/model_core.cpp:21-109; src/model/model.cpp:44-75,111-150.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestModelNativeDictionaryGrowthCopyOrderAndReinsert(t *testing.T) {
	m := astTestManager(t)
	original, e := model.NewModel(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, original.Close()) })
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	astTestPin(t, m, yes)
	astTestPin(t, m, no)
	const count = 40
	constants := make([]ast.ASTHandle, count)
	functions := make([]ast.ASTHandle, count)
	sorts := make([]ast.ASTHandle, count)
	values := make([]ast.ASTHandle, count)
	graphs := make([]*model.FunctionInterpretation, count)
	for i := 0; i < count; i++ {
		constants[i] = modelTestDecl(t, m, fmt.Sprint("native-constant-", i))
		functions[i] = modelTestFunctionDecl(t, m, fmt.Sprint("native-function-", i))
		graphs[i] = modelTestFunction(t, m, 1)
		astTestMust(t, graphs[i].SetElse(yes))
		sorts[i], e = m.MkUninterpretedSort(astTestSymbol(t, m, fmt.Sprint("native-sort-", i)), nil)
		astTestMust(t, e)
		values[i], e = m.GetSomeValue(sorts[i])
		astTestMust(t, e)
		astTestPin(t, m, values[i])
	}

	// Grow all three dictionaries with actual ASTs/graphs already constructed.
	// Native collection growth must not create numeric-arena reservations.
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	t.Cleanup(func() { m.Budget().FailAfter(-1) })
	for i := 0; i < count; i++ {
		astTestMust(t, original.Core().RegisterConstant(constants[i], yes))
		astTestMust(t, original.Core().RegisterFunction(functions[i], graphs[i]))
		astTestMust(t, original.RegisterUniverse(sorts[i], []ast.ASTHandle{values[i]}))
	}
	astTestMust(t, original.Core().RegisterConstant(constants[0], no))
	astTestMust(t, original.Core().RegisterFunction(functions[0], graphs[0]))
	if m.Budget().Used() != before {
		t.Fatal("native dictionaries charged numeric storage", before, m.Budget().Used())
	}
	m.Budget().FailAfter(-1)

	// copy_const_interps and copy_func_interps walk the source's m_interp
	// and m_finterp in table order, so the copy's declarations follow it.
	const retired = 7
	astTestMust(t, original.Core().UnregisterDeclaration(constants[retired]))
	astTestMust(t, original.Core().RegisterConstant(constants[retired], yes))
	astTestMust(t, original.Core().UnregisterDeclaration(functions[retired]))
	replacement := modelTestFunction(t, m, 1)
	astTestMust(t, replacement.SetElse(no))
	astTestMust(t, original.Core().RegisterFunction(functions[retired], replacement))
	constantOrder := original.Core().ConstantDeclarationsInTableOrderForTest()
	functionOrder, e := original.Core().FunctionDeclarationsInTableOrder()
	astTestMust(t, e)
	if len(constantOrder) != count || len(functionOrder) != count {
		t.Fatal("table order lost declarations", len(constantOrder), len(functionOrder))
	}
	indexOf := make(map[ast.ASTHandle]int, count)
	for i := 0; i < count; i++ {
		indexOf[functions[i]] = i
	}
	copied, e := original.Copy()
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, copied.Close()) })
	astTestMust(t, original.Close())
	for position := 0; position < count; position++ {
		d, e := copied.Core().Constant(uint32(position))
		astTestMust(t, e)
		if d != constantOrder[position] {
			t.Fatal("constant copy did not follow m_interp order", position)
		}
		d, e = copied.Core().Function(uint32(position))
		astTestMust(t, e)
		if d != functionOrder[position] {
			t.Fatal("function copy did not follow m_finterp order", position)
		}
		index := indexOf[d]
		fi, e := copied.Core().FunctionInterpretation(d)
		astTestMust(t, e)
		fallback, e := fi.Else()
		astTestMust(t, e)
		want := yes
		if index == retired {
			want = no
		}
		if fi == graphs[index] || fallback != want {
			t.Fatal("copy lost independent function ownership", position)
		}
		s, e := copied.UninterpretedSort(uint32(position))
		astTestMust(t, e)
		if s != sorts[position] {
			t.Fatal("universe copy changed insertion order", position)
		}
		u, e := copied.Universe(s)
		astTestMust(t, e)
		if len(u) != 1 || u[0] != values[position] {
			t.Fatal("copy lost universe ownership", position)
		}
	}
	v, e := copied.Core().ConstantInterpretation(constants[0])
	astTestMust(t, e)
	if v != no {
		t.Fatal("copy lost replaced constant value")
	}
}
