package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPBPreprocessDoesNotEliminateVariableUsedOutsideFragment(t *testing.T) {
	f := newPBPreprocessFixture(t)
	pb := f.ge(f.vars[:1], []string{"1"}, "1")
	name := pbPreprocessSymbol(t, f.m, "pb-unhandled")
	b, e := f.m.MkBoolSort()
	simplifyMust(t, e)
	fn, e := f.m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	simplifyMust(t, e)
	other, e := f.m.MkApp(fn, []ast.ASTHandle{f.vars[0]})
	simplifyMust(t, e)
	g := f.goal(pb, other)
	f.apply(g)
	if n, e := g.Size(); e != nil || n != 2 {
		t.Fatal("declassified variable was eliminated", n, e)
	}
}
