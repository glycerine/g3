package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVArray2UFSelectAndArrayEquality(t *testing.T) {
	f := newArrayFixture(t)
	selectTerm := f.app(t, ast.ASTArrayOpSelect, f.a, f.i)
	rewritten, e := f.tactic.rewriter.rewrite(selectTerm)
	arrayMust(t, e)
	if yes, e := f.tactic.rewriter.isBVArray(rewritten); e != nil || yes {
		t.Fatal("select retained array", yes, e)
	}
	d, e := f.m.GetDecl(rewritten)
	arrayMust(t, e)
	if d != f.tactic.rewriter.arrays[f.a] {
		t.Fatal("select did not use array function")
	}
	eq, e := f.m.MkEq(f.a, f.b)
	arrayMust(t, e)
	q, e := f.tactic.rewriter.rewrite(eq)
	arrayMust(t, e)
	if k, e := f.m.GetKind(q); e != nil || k != ast.ASTQuantifier {
		t.Fatal("array equality not extensional", k, e)
	}
}
func TestBVArray2UFSameArrayEqualityIsTrue(t *testing.T) {
	f := newArrayFixture(t)
	eq, e := f.m.MkEq(f.a, f.a)
	arrayMust(t, e)
	got, e := f.tactic.rewriter.rewrite(eq)
	arrayMust(t, e)
	truth, e := f.m.MkTrue()
	arrayMust(t, e)
	if got != truth {
		t.Fatal(got, truth)
	}
}
