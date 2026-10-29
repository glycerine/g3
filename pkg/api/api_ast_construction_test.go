package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestSMTContextASTConstruction(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	u := f.pin(f.c.UninterpretedSort("U"))
	boolean := f.pin(f.c.BoolSort())
	fn := f.pin(f.c.Function("f", []ast.ASTHandle{u}, boolean))
	x := f.pin(f.c.Constant("x", u))
	fx := f.pin(f.c.Apply(fn, x))
	if sort := f.pin(f.c.SortOf(fx)); sort != boolean {
		t.Fatal("application sort", sort, boolean)
	}
	truth := f.pin(f.c.Bool(true))
	falsehood := f.pin(f.c.Bool(false))
	terms := []struct {
		kind int32
		term ast.ASTHandle
	}{{ast.ASTOpNot, f.pin(f.c.Not(truth))}, {ast.ASTOpEq, f.pin(f.c.Equal(x, x))}, {ast.ASTOpDistinct, f.pin(f.c.Distinct(x, x))}, {ast.ASTOpIff, f.pin(f.c.Iff(truth, falsehood))}, {ast.ASTOpImplies, f.pin(f.c.Implies(truth, falsehood))}, {ast.ASTOpXor, f.pin(f.c.Xor(truth, falsehood))}, {ast.ASTOpAnd, f.pin(f.c.And(truth, falsehood))}, {ast.ASTOpOr, f.pin(f.c.Or(truth, falsehood))}, {ast.ASTOpIte, f.pin(f.c.Ite(truth, x, x))}}
	for _, term := range terms {
		if kind, err := f.c.DeclarationKind(term.term); err != nil || kind != term.kind {
			t.Fatal("kind", term.kind, kind, err)
		}
	}
	freshDecl := f.pin(f.c.FreshFunction("fresh", []ast.ASTHandle{u}, boolean))
	freshConst := f.pin(f.c.FreshConstant("fresh", u))
	if freshDecl == fn || freshConst == x {
		t.Fatal("fresh identity")
	}
}

func TestSMTContextASTSymbols(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	numeric, err := f.c.IntegerSymbol(17)
	astTestMust(t, err)
	if text, err := f.c.SymbolString(numeric); err != nil || text != "17" {
		t.Fatal(text, err)
	}
	textSymbol, err := f.c.StringSymbol("hello")
	astTestMust(t, err)
	if text, err := f.c.SymbolString(textSymbol); err != nil || text != "hello" {
		t.Fatal(text, err)
	}
}
