package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestModelFinderHintDefinitionCycleFixedPointAndAnnotations(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		astTestMust(t, m.IncRef(h))
		t.Cleanup(func() { astTestMust(t, m.DecRef(h)) })
		return h
	}
	decl := make([]ast.ASTHandle, 3)
	app := make([]ast.ASTHandle, 3)
	for i := range decl {
		decl[i], e = m.MkFuncDecl(symbol.Numerical(uint32(100+i)), nil, b, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		pin(decl[i])
		app[i], e = m.MkApp(decl[i], nil)
		astTestMust(t, e)
		pin(app[i])
	}
	v, e := m.MkVar(0, b)
	astTestMust(t, e)
	pin(v)

	s := newHintSearchState(m)
	astTestMust(t, s.selectDefinition(decl[0], app[1]))
	astTestMust(t, s.selectDefinition(decl[1], app[2]))
	astTestMust(t, s.selectDefinition(decl[2], v))
	if cycle, e := s.isCyclic(m); e != nil || cycle {
		t.Fatal("fixed point must admit f -> g -> h -> variable", cycle, e)
	}
	if e := s.selectDefinition(decl[2], app[0]); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal("m_fs duplicate assertion", e)
	}
	s.forgetDefinition(decl[2])
	astTestMust(t, s.selectDefinition(decl[2], app[0]))
	if cycle, e := s.isCyclic(m); e != nil || !cycle {
		t.Fatal("three-function cycle", cycle, e)
	}
	s.forgetDefinition(decl[1])
	if cycle, e := s.isCyclic(m); e != nil || cycle {
		t.Fatal("unselected g breaks the cycle", cycle, e)
	}
	wrapperDecl, e := m.MkFuncDecl(symbol.Numerical(200), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	pin(wrapperDecl)
	wrapped, e := m.MkApp(wrapperDecl, []ast.ASTHandle{app[1]})
	astTestMust(t, e)
	pin(wrapped)
	s = newHintSearchState(m)
	astTestMust(t, s.selectDefinition(decl[0], wrapped))
	astTestMust(t, s.selectDefinition(decl[1], app[0]))
	if cycle, e := s.isCyclic(m); e != nil || !cycle {
		t.Fatal("selected function under unselected application", cycle, e)
	}

	patternDecl, e := m.MkFuncDecl(symbol.Numerical(201), []ast.ASTHandle{b, b}, b, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	pin(patternDecl)
	patternTerm, e := m.MkApp(patternDecl, []ast.ASTHandle{app[1], v})
	astTestMust(t, e)
	pin(patternTerm)
	pattern, e := m.MkFamilyApp(2, 0, nil, []ast.ASTHandle{patternTerm})
	astTestMust(t, e)
	pin(pattern)
	qPattern, e := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{symbol.Numerical(1)},
		Body: v, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, e)
	pin(qPattern)
	s = newHintSearchState(m)
	astTestMust(t, s.selectDefinition(decl[0], qPattern))
	astTestMust(t, s.selectDefinition(decl[1], app[0]))
	if cycle, e := s.isCyclic(m); e != nil || !cycle {
		t.Fatal("quantifier pattern occurrence", cycle, e)
	}
	qNoPattern, e := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{symbol.Numerical(2)},
		Body: v, NoPatterns: []ast.ASTHandle{app[1]},
	})
	astTestMust(t, e)
	pin(qNoPattern)
	s.forgetDefinition(decl[0])
	astTestMust(t, s.selectDefinition(decl[0], qNoPattern))
	if cycle, e := s.isCyclic(m); e != nil || !cycle {
		t.Fatal("quantifier no-pattern occurrence", cycle, e)
	}
	s.forgetDefinition(decl[1])
	if cycle, e := s.isCyclic(m); e != nil || cycle {
		t.Fatal("unselected annotation function", cycle, e)
	}
}
