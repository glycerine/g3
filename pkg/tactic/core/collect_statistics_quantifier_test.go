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
	patternTerm := f.app(t, patternDecl, x)
	pattern, e := f.m.MkPattern([]ast.ASTHandle{patternTerm})
	simplifyMust(t, e)
	simplifyPin(t, f.m, pattern)
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

func TestCollectStatisticsNestedQuantifierCountsAndSourceDepth(t *testing.T) {
	f := newCollectStatisticsFixture(t)
	r := f.runner(t)
	boolean, err := f.m.MkBoolSort()
	simplifyMust(t, err)

	predName, err := f.m.Symbols.Intern("stats-nested-predicate")
	simplifyMust(t, err)
	pred, err := f.m.MkFuncDecl(predName, []ast.ASTHandle{f.sort}, boolean, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	simplifyPin(t, f.m, pred)

	x, err := f.m.MkVar(0, f.sort)
	simplifyMust(t, err)
	innerBody := f.app(t, pred, x)
	patternName, err := f.m.Symbols.Intern("stats-nested-pattern")
	simplifyMust(t, err)
	patternDecl, err := f.m.MkFuncDecl(patternName, []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	simplifyPin(t, f.m, patternDecl)
	patternTerm := f.app(t, patternDecl, x)
	pattern, e := f.m.MkPattern([]ast.ASTHandle{patternTerm})
	simplifyMust(t, e)
	simplifyPin(t, f.m, pattern)
	noPatternName, err := f.m.Symbols.Intern("stats-nested-no-pattern")
	simplifyMust(t, err)
	noPatternDecl, err := f.m.MkFuncDecl(noPatternName, []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	simplifyPin(t, f.m, noPatternDecl)
	noPattern := f.app(t, noPatternDecl, x)

	innerName, err := f.m.Symbols.Intern("stats-nested-exists")
	simplifyMust(t, err)
	inner, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: false, Sorts: []ast.ASTHandle{f.sort}, Names: []usymbol.Symbol{innerName},
		Body: innerBody, Patterns: []ast.ASTHandle{pattern},
	})
	simplifyMust(t, err)

	wrapName, err := f.m.Symbols.Intern("stats-nested-wrap")
	simplifyMust(t, err)
	wrap, err := f.m.MkFuncDecl(wrapName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	simplifyPin(t, f.m, wrap)
	outerBody := f.app(t, wrap, inner)
	outerName, err := f.m.Symbols.Intern("stats-nested-forall")
	simplifyMust(t, err)
	outer, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{f.sort}, Names: []usymbol.Symbol{outerName}, Body: outerBody,
		NoPatterns: []ast.ASTHandle{noPattern},
	})
	simplifyMust(t, err)

	applyStatistics(t, r, f.goal(t, outer))
	s, err := r.Snapshot()
	simplifyMust(t, err)
	for k, want := range map[string]uint64{
		"quantifiers": 2, "forall-variables": 1, "exists-variables": 1,
		"patterns": 1, "no-patterns": 1, "max-quantification-depth": 1,
	} {
		if s[k] != want {
			t.Fatalf("%s = %d, want %d (all stats: %v)", k, s[k], want, s)
		}
	}
}
