package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// The preserved C++ driver has no focused assertions for the ITE and
// distinct branches in bvarray2uf_rewriter_cfg::reduce_app.  These tests use
// independent structural expectations from the transformation in [1].
func TestBVArray2UFArrayITEAddsPointwiseDefinition(t *testing.T) {
	f := newArrayFixture(t)
	boolean, err := f.m.MkBoolSort()
	arrayMust(t, err)
	name, err := f.m.Symbols.Intern("array-condition")
	arrayMust(t, err)
	condition, err := f.m.MkConst(name, boolean)
	arrayMust(t, err)
	value, err := f.m.MkIte(condition, f.a, f.b)
	arrayMust(t, err)

	rewritten, err := f.tactic.rewriter.rewrite(value)
	arrayMust(t, err)
	if _, err = f.m.AsArrayFunction(rewritten); err != nil {
		t.Fatalf("array ITE result is not an as-array term: %v", err)
	}
	if len(f.tactic.rewriter.extra) != 1 {
		t.Fatalf("array ITE pointwise definitions = %d, want 1", len(f.tactic.rewriter.extra))
	}
	if kind, err := f.m.GetKind(f.tactic.rewriter.extra[0]); err != nil || kind != ast.ASTQuantifier {
		t.Fatalf("array ITE definition kind = %v, %v", kind, err)
	}
}

func TestBVArray2UFDistinctExpandsBeforeArrayElimination(t *testing.T) {
	f := newArrayFixture(t)
	distinct, err := f.m.MkDistinct([]ast.ASTHandle{f.a, f.b})
	arrayMust(t, err)
	rewritten, err := f.tactic.rewriter.rewrite(distinct)
	arrayMust(t, err)
	decl, err := f.m.GetDecl(rewritten)
	arrayMust(t, err)
	op, err := f.m.GetDeclKind(decl)
	arrayMust(t, err)
	if op == ast.ASTOpDistinct {
		t.Fatal("array distinct survived its required pairwise expansion")
	}
}

func TestBVArray2UFSelectFromAsArrayReusesFunction(t *testing.T) {
	f := newArrayFixture(t)
	name, err := f.m.Symbols.Intern("source-array-function")
	arrayMust(t, err)
	function, err := f.m.MkFuncDecl(name, []ast.ASTHandle{f.index}, f.value, ast.ASTFuncDeclInfo{})
	arrayMust(t, err)
	array, err := f.m.MkAsArray(function)
	arrayMust(t, err)
	selected := f.app(t, ast.ASTArrayOpSelect, array, f.i)
	rewritten, err := f.tactic.rewriter.rewrite(selected)
	arrayMust(t, err)
	decl, err := f.m.GetDecl(rewritten)
	arrayMust(t, err)
	if decl != function {
		t.Fatalf("select declaration = %v, want original %v", decl, function)
	}
	if len(f.tactic.rewriter.declarations) != 0 {
		t.Fatal("as-array selection allocated a replacement function")
	}
}
