package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBVArray2UFCachePersistsCleanupAndTranslationFresh(t *testing.T) {
	f := newArrayFixture(t)
	selectTerm := f.app(t, ast.ASTArrayOpSelect, f.a, f.i)
	_, e := f.tactic.rewriter.rewrite(selectTerm)
	arrayMust(t, e)
	decl := f.tactic.rewriter.arrays[f.a]
	_, e = f.tactic.rewriter.rewrite(selectTerm)
	arrayMust(t, e)
	if f.tactic.rewriter.arrays[f.a] != decl || len(f.tactic.rewriter.declarations) != 1 {
		t.Fatal("cache miss")
	}
	copyT, e := f.tactic.Translate(f.m)
	arrayMust(t, e)
	copy := copyT.(*BVArray2UFTactic)
	t.Cleanup(func() { arrayMust(t, copy.Close()) })
	if len(copy.rewriter.arrays) != 0 {
		t.Fatal("translation copied cache")
	}
	arrayMust(t, f.tactic.Cleanup())
	if len(f.tactic.rewriter.arrays) != 0 || len(f.tactic.rewriter.extra) != 0 {
		t.Fatal("cleanup retained rewrite state")
	}
}
