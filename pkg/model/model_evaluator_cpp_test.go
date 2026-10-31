package model_test

// Source: src/test/model_evaluator.cpp.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestModelEvaluatorSourceNestedFunctionInterpretations(t *testing.T) {
	m, md, ev, _, _, _ := evaluatorFixture(t)
	one, e := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, e)
	astTestPin(t, m, one)
	two, e := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, e)
	astTestPin(t, m, two)
	intSort, e := m.GetSort(one)
	astTestMust(t, e)
	boolSort, e := m.MkBoolSort()
	astTestMust(t, e)
	decl := func(name string) ast.ASTHandle {
		h, e := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{intSort, boolSort}, intSort, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	f, g, h, F, G := decl("f"), decl("g"), decl("h"), decl("F"), decl("G")
	variable := func(i uint32, s ast.ASTHandle) ast.ASTHandle {
		v, e := m.MkVar(i, s)
		astTestMust(t, e)
		astTestPin(t, m, v)
		return v
	}
	i0, i1, b1, b2 := variable(0, intSort), variable(1, intSort), variable(1, boolSort), variable(2, boolSort)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		v, e := m.MkApp(d, args)
		astTestMust(t, e)
		astTestPin(t, m, v)
		return v
	}
	ite := func(c, a, b ast.ASTHandle) ast.ASTHandle {
		v, e := m.MkIte(c, a, b)
		astTestMust(t, e)
		astTestPin(t, m, v)
		return v
	}
	fi, e := model.NewFunctionInterpretation(m, 2)
	astTestMust(t, e)
	gi, e := model.NewFunctionInterpretation(m, 2)
	astTestMust(t, e)
	hi, e := model.NewFunctionInterpretation(m, 2)
	astTestMust(t, e)
	astTestMust(t, hi.SetElse(ite(b1, app(f, i0, b1), app(g, i0, b1))))
	astTestMust(t, md.Core().RegisterFunction(h, hi))
	exists := func(d ast.ASTHandle) ast.ASTHandle {
		le, err := m.MkArithmeticLE(i0, app(d, i1, b2))
		astTestMust(t, err)
		astTestPin(t, m, le)
		name, err := m.Symbols.Intern("N")
		astTestMust(t, err)
		v, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{intSort}, Names: []symbol.Symbol{name}, Body: le})
		astTestMust(t, err)
		astTestPin(t, m, v)
		return v
	}
	astTestMust(t, fi.SetElse(ite(exists(F), i0, one)))
	astTestMust(t, gi.SetElse(ite(exists(G), two, i0)))
	astTestMust(t, md.Core().RegisterFunction(g, gi))
	astTestMust(t, md.Core().RegisterFunction(f, fi))
	input := app(h, i0, b1)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, ev.Evaluate(input, out))
	first, e := m.RefGet(out)
	astTestMust(t, e)
	if first == input {
		t.Fatal("nested quantified interpretations were not expanded")
	}
	quantified, e := m.HasQuantifiers(first)
	astTestMust(t, e)
	if !quantified {
		t.Fatal("nested interpretation lost its quantifiers")
	}
	astTestMust(t, fi.SetElse(app(F, i0, b1)))
	astTestMust(t, gi.SetElse(app(G, i0, b1)))
	astTestMust(t, md.Core().RegisterFunction(g, gi))
	astTestMust(t, md.Core().RegisterFunction(h, hi))
	astTestMust(t, ev.Evaluate(input, out))
	second, e := m.RefGet(out)
	astTestMust(t, e)
	expected := ite(b1, app(F, i0, b1), app(G, i0, b1))
	if second != expected {
		t.Fatal("modified nested interpretations were not reflected", second, expected)
	}
}
