package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestCollectStatisticsQuantifierVariablesPatternsAndDepth(t *testing.T) {
	f := newCollectStatisticsFixture(t)
	r := f.runner(t)
	boolean, e := f.m.MkBoolSort()
	simplifyMust(t, e)
	pn, e := f.m.Symbols.Intern("stats-p")
	simplifyMust(t, e)
	p, e := f.m.MkFuncDecl(pn, []ast.ASTHandle{f.sort}, boolean, ast.ASTFuncDeclInfo{})
	simplifyMust(t, e)
	simplifyPin(t, f.m, p)
	x, e := f.m.MkVar(0, f.sort)
	simplifyMust(t, e)
	body := f.app(t, p, x)
	patternDeclName, e := f.m.Symbols.Intern("stats-pattern")
	simplifyMust(t, e)
	patternDecl, e := f.m.MkFuncDecl(patternDeclName, []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
	simplifyMust(t, e)
	simplifyPin(t, f.m, patternDecl)
	pattern := f.app(t, patternDecl, x)
	name, e := f.m.Symbols.Intern("stats-x")
	simplifyMust(t, e)
	q, e := f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.sort}, Names: []usymbol.Symbol{name}, Body: body, Patterns: []ast.ASTHandle{pattern}})
	simplifyMust(t, e)
	applyStatistics(t, r, f.goal(t, q))
	s, e := r.Snapshot()
	simplifyMust(t, e)
	for k, v := range map[string]uint64{"quantifiers": 1, "forall-variables": 1, "patterns": 1, "no-patterns": 0, "max-quantification-depth": 1, "bound-variables": 1} {
		if s[k] != v {
			t.Fatal(k, s[k], v, s)
		}
	}
}
