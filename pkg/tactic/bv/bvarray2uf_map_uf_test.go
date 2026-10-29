package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVArray2UFMapAddsPointwiseDefinition(t *testing.T) {
	f := newArrayFixture(t)
	name, e := f.m.Symbols.Intern("map-value")
	arrayMust(t, e)
	fn, e := f.m.MkFuncDecl(name, []ast.ASTHandle{f.value, f.value}, f.value, ast.ASTFuncDeclInfo{})
	arrayMust(t, e)
	mapped, e := f.m.MkArrayMap(fn, ast.SliceArguments([]ast.ASTHandle{f.a, f.b}))
	arrayMust(t, e)
	got, e := f.tactic.rewriter.rewrite(mapped)
	arrayMust(t, e)
	if _, e = f.m.AsArrayFunction(got); e != nil {
		t.Fatal(e)
	}
	if len(f.tactic.rewriter.extra) != 1 {
		t.Fatal(len(f.tactic.rewriter.extra))
	}
}
func TestBVArray2UFUninterpretedArrayResultBecomesAsArray(t *testing.T) {
	f := newArrayFixture(t)
	name, e := f.m.Symbols.Intern("array-f")
	arrayMust(t, e)
	fn, e := f.m.MkFuncDecl(name, []ast.ASTHandle{f.index}, f.array, ast.ASTFuncDeclInfo{})
	arrayMust(t, e)
	term, e := f.m.MkApp(fn, []ast.ASTHandle{f.i})
	arrayMust(t, e)
	got, e := f.tactic.rewriter.rewrite(term)
	arrayMust(t, e)
	if _, e = f.m.AsArrayFunction(got); e != nil {
		t.Fatal(e)
	}
}
