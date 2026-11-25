package ast

import (
	"errors"
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestExpressionHeaderAndResultSort(t *testing.T) {
	m := astTestManager(t)
	boolean, e := m.MkBoolSort()
	astTestMust(t, e)
	v, e := m.MkVar(0, boolean)
	astTestMust(t, e)
	astTestPin(t, m, v)
	pred, e := m.MkFuncDecl(astTestSymbol(t, m, "header-p"), []ASTHandle{boolean}, boolean, ASTFuncDeclInfo{})
	astTestMust(t, e)
	body, e := m.MkApp(pred, []ASTHandle{v})
	astTestMust(t, e)
	astTestPin(t, m, body)
	q, e := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{boolean}, Names: []usymbol.Symbol{astTestSymbol(t, m, "x")}, Body: body})
	astTestMust(t, e)
	astTestPin(t, m, q)
	for _, h := range []ASTHandle{body, v, q} {
		want, e := m.NodeHeader(h)
		astTestMust(t, e)
		header, yes, e := m.ExpressionHeader(h)
		if e != nil || header != want || !yes {
			t.Fatalf("header %+v boolean %t error %v; want %+v", header, yes, e, want)
		}
	}
	for _, h := range []ASTHandle{pred, boolean} {
		_, _, e := m.ExpressionHeader(h)
		if !errors.Is(e, ErrASTDomain) {
			t.Fatal("non-expression", e)
		}
	}
	foreign := astTestManager(t)
	constant, e := foreign.MkTrue()
	astTestMust(t, e)
	if _, _, e = m.ExpressionHeader(constant); !errors.Is(e, ErrASTHandle) {
		t.Fatal("foreign manager", e)
	}
}
