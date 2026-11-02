package model_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestModel2ExprFreshNameConcreteSymbolSetAndSequence(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	aName := astTestSymbol(t, m, "A")
	a, e := m.MkFuncDecl(aName, nil, b, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, a)
	cName := astTestSymbol(t, m, "C")
	c, e := m.MkFuncDecl(cName, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, c)
	v, e := m.MkVar(0, b)
	astTestMust(t, e)
	astTestPin(t, m, v)
	body, e := m.MkApp(c, []ast.ASTHandle{v})
	astTestMust(t, e)
	astTestPin(t, m, body)
	bName := astTestSymbol(t, m, "B")
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{bName}, Body: body})
	astTestMust(t, e)
	astTestPin(t, m, q)
	fresh := model.NewFreshName(m)
	astTestMust(t, fresh.AddAST(a))
	astTestMust(t, fresh.AddAST(q))
	for _, s := range []string{"A", "B", "C"} {
		if !fresh.Contains(astTestSymbol(t, m, s)) {
			t.Fatal("missing source name", s)
		}
	}
	fresh.AddSymbol(astTestSymbol(t, m, "A1"))
	for i := byte('E'); i <= 'Z'; i++ {
		got, e := fresh.Next()
		astTestMust(t, e)
		spelling, e := m.Symbols.String(got)
		astTestMust(t, e)
		if spelling != string(i) {
			t.Fatal("source alphabet sequence", spelling, string(i))
		}
	}
	got, e := fresh.Next()
	astTestMust(t, e)
	spelling, e := m.Symbols.String(got)
	astTestMust(t, e)
	if spelling != "C1" {
		t.Fatal("source numbered collision skip", spelling)
	}
	pName := astTestSymbol(t, m, "pattern-name")
	pDecl, e := m.MkFuncDecl(pName, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, pDecl)
	pattern, e := m.MkApp(pDecl, []ast.ASTHandle{v})
	astTestMust(t, e)
	astTestPin(t, m, pattern)
	annotation, e := m.MkFamilyApp(2, 0, nil, []ast.ASTHandle{pattern})
	astTestMust(t, e)
	astTestPin(t, m, annotation)
	for _, annotations := range []struct{ pattern, noPattern bool }{{pattern: true}, {noPattern: true}} {
		spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{bName}, Body: body}
		if annotations.pattern {
			spec.Patterns = []ast.ASTHandle{annotation}
		} else {
			spec.NoPatterns = []ast.ASTHandle{pattern}
		}
		annotated, e := m.MkQuantifier(spec)
		astTestMust(t, e)
		astTestPin(t, m, annotated)
		collector := model.NewFreshName(m)
		astTestMust(t, collector.AddAST(annotated))
		if !collector.Contains(pName) {
			t.Fatal("annotation declaration not visited", annotations)
		}
	}
}

// Source model2expr.cpp increments inside the body and again at the loop
// boundary on rejection. Returned candidates execute only the first increment.
func TestModel2ExprFreshNameOccupiedCandidateSourceAdvance(t *testing.T) {
	for _, tc := range []struct {
		used []string
		want []string
	}{
		{nil, []string{"A", "B", "C"}},
		{[]string{"A"}, []string{"C", "D", "E"}},
		{[]string{"B"}, []string{"A", "D", "E"}},
		{[]string{"A", "C"}, []string{"E", "F", "G"}},
	} {
		m := astTestManager(t)
		fresh := model.NewFreshName(m)
		for _, name := range tc.used {
			fresh.AddSymbol(astTestSymbol(t, m, name))
		}
		for _, want := range tc.want {
			got, err := fresh.Next()
			astTestMust(t, err)
			text, err := m.Symbols.String(got)
			astTestMust(t, err)
			if text != want {
				t.Fatal(tc.used, text, want)
			}
			if fresh.Contains(got) {
				t.Fatal("next unexpectedly reserved a generated name", text)
			}
		}
	}
}
