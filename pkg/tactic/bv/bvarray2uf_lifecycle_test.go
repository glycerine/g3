package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBVArray2UFCachePersistsCleanupAndTranslationFresh(t *testing.T) {
	f := newArrayFixture(t)
	selectTerm := f.app(t, ast.ASTArrayOpSelect, f.a, f.i)
	_, _, e := f.tactic.rewriter.rewrite(selectTerm)
	arrayMust(t, e)
	decl := arrayDecl(t, f.tactic.rewriter, f.a)
	_, _, e = f.tactic.rewriter.rewrite(selectTerm)
	arrayMust(t, e)
	if arrayDecl(t, f.tactic.rewriter, f.a) != decl || f.tactic.rewriter.arrays.Len() != 1 {
		t.Fatal("cache miss")
	}
	copyT, e := f.tactic.Translate(f.m)
	arrayMust(t, e)
	copy := copyT.(*BVArray2UFTactic)
	t.Cleanup(func() { arrayMust(t, copy.Close()) })
	if copy.rewriter.arrays.Len() != 0 {
		t.Fatal("translation copied cache")
	}
	arrayMust(t, f.tactic.Cleanup())
	if f.tactic.rewriter.arrays.Len() != 0 || len(f.tactic.rewriter.extra) != 0 {
		t.Fatal("cleanup retained rewrite state")
	}
}
