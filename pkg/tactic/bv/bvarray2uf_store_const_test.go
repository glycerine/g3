package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVArray2UFStoreAddsUniversalAndGroundAxioms(t *testing.T) {
	f := newArrayFixture(t)
	store := f.app(t, ast.ASTArrayOpStore, f.a, f.i, f.v)
	got, e := f.tactic.rewriter.rewrite(store)
	arrayMust(t, e)
	if _, e = f.m.AsArrayFunction(got); e != nil {
		t.Fatal("store result is not as-array", e)
	}
	if len(f.tactic.rewriter.extra) != 2 {
		t.Fatal("store side constraints", len(f.tactic.rewriter.extra))
	}
	if k, e := f.m.GetKind(f.tactic.rewriter.extra[0]); e != nil || k != ast.ASTQuantifier {
		t.Fatal(k, e)
	}
}
func TestBVArray2UFConstAddsPointwiseDefinition(t *testing.T) {
	f := newArrayFixture(t)
	constant, e := f.m.MkConstArray(f.array, f.v)
	arrayMust(t, e)
	got, e := f.tactic.rewriter.rewrite(constant)
	arrayMust(t, e)
	if _, e = f.m.AsArrayFunction(got); e != nil {
		t.Fatal(e)
	}
	if len(f.tactic.rewriter.extra) != 1 {
		t.Fatal(len(f.tactic.rewriter.extra))
	}
}
