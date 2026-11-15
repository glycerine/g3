package rewriter

// Source: src/ast/rewriter/der.cpp:149-155.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// AUDIT.md ast_rewriter_03-F1: der::reduce1 in 4.7.1 leaves existential
// conjunctions alone.
func TestDERExistentialConjunctionLeftUnchanged(t *testing.T) {
	f := newFactorFixture(t)
	d, err := NewDER(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, d.Close()) })
	x, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	y, err := f.m.MkVar(1, f.sort)
	astTestMust(t, err)
	equality, err := f.m.MkEq(x, y)
	astTestMust(t, err)
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	predicate, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "der-exists-p"), []ast.ASTHandle{f.sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	px, err := f.m.MkApp(predicate, []ast.ASTHandle{x})
	astTestMust(t, err)
	body, err := f.m.MkAndArray([]ast.ASTHandle{equality, px})
	astTestMust(t, err)
	q, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{f.sort, f.sort}, Names: []usymbol.Symbol{astTestSymbol(t, f.m, "x"), astTestSymbol(t, f.m, "y")}, Body: body, NoPatterns: []ast.ASTHandle{px}})
	astTestMust(t, err)
	if got := derReduce(t, d, f.m, q); got != q {
		t.Fatal("existential conjunction changed", got, q)
	}
}
