package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPBPreprocessSpecialResolutionEliminatesUnitSide(t *testing.T) {
	f := newPBPreprocessFixture(t)
	first, e := f.m.MkOr(f.vars[0], f.vars[1])
	simplifyMust(t, e)
	nx, e := f.m.MkNot(f.vars[0])
	simplifyMust(t, e)
	ny, e := f.m.MkNot(f.vars[1])
	simplifyMust(t, e)
	second := f.ge([]ast.ASTHandle{nx, ny, f.vars[2]}, []string{"2", "3", "1"}, "3")
	name := pbPreprocessSymbol(t, f.m, "keep-z")
	b, e := f.m.MkBoolSort()
	simplifyMust(t, e)
	fn, e := f.m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	simplifyMust(t, e)
	other, e := f.m.MkApp(fn, []ast.ASTHandle{f.vars[2]})
	simplifyMust(t, e)
	g := f.goal(first, second, other)
	f.apply(g)
	n, e := g.Size()
	simplifyMust(t, e)
	if n >= 3 {
		t.Fatal("special resolvent was not published", n)
	}
}
