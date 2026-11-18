package pattern

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExprPatternMatcherCompiledDeclarationAndBoundInstantiation(t *testing.T) {
	m := patternManager(t)
	b, err := m.MkBoolSort()
	patternMust(t, err)
	variableDecl := func(name string) ast.ASTHandle {
		t.Helper()
		s, e := m.Symbols.Intern(name)
		patternMust(t, e)
		d, e := m.MkFuncDecl(s, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
		patternMust(t, e)
		return d
	}
	patternDecl, actualDecl := variableDecl("?function"), variableDecl("actual-function")
	v0, err := m.MkVar(0, b)
	patternMust(t, err)
	patternApp, err := m.MkApp(patternDecl, []ast.ASTHandle{v0})
	patternMust(t, err)
	templateBody, err := m.MkIff(patternApp, v0)
	patternMust(t, err)
	annotation, err := m.MkPattern([]ast.ASTHandle{patternApp})
	patternMust(t, err)
	template, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{symbol.Numerical(1)}, Body: templateBody, Weight: 7, Patterns: []ast.ASTHandle{annotation}})
	patternMust(t, err)
	actualApp, err := m.MkApp(actualDecl, []ast.ASTHandle{v0})
	patternMust(t, err)
	actualBody, err := m.MkIff(v0, actualApp) // exercise the commutative retry
	patternMust(t, err)
	actual, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{symbol.Numerical(2)}, Body: actualBody})
	patternMust(t, err)
	for _, h := range []ast.ASTHandle{patternDecl, actualDecl, template, actual} {
		patternMust(t, m.IncRef(h))
		h := h
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
	}
	matcher, err := newExprPatternMatcher(m)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, matcher.Close()) })
	patternMust(t, matcher.Compile(template))
	patterns, weight, matched, err := matcher.MatchQuantifier(actual)
	patternMust(t, err)
	if !matched || weight != 7 || len(patterns) != 1 {
		t.Fatal("compiled match", matched, weight, len(patterns))
	}
	defer func() { patternMust(t, m.DelRef(&patterns[0])) }()
	gotPattern, err := m.RefGet(patterns[0])
	patternMust(t, err)
	gotTerm, err := m.GetArg(gotPattern, 0)
	patternMust(t, err)
	if gotTerm != actualApp {
		t.Fatal("instantiated declaration or bound variable", gotTerm, actualApp)
	}
	copyOwner := *matcher
	if err = copyOwner.Compile(template); err != ast.ErrASTHandle {
		t.Fatal("copied matcher owner", err)
	}
}

func TestExprPatternMatcherEmbeddedDatabase(t *testing.T) {
	m := patternManager(t)
	arith, err := m.RegisterArithmeticPlugin()
	patternMust(t, err)
	integer, err := m.MkFamilySort(arith, 1, nil)
	patternMust(t, err)
	decl := func(name string, arity int) ast.ASTHandle {
		t.Helper()
		s, e := m.Symbols.Intern(name)
		patternMust(t, e)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = integer
		}
		d, e := m.MkFuncDecl(s, domain, integer, ast.ASTFuncDeclInfo{})
		patternMust(t, e)
		patternMust(t, m.IncRef(d))
		t.Cleanup(func() { patternMust(t, m.DecRef(d)) })
		return d
	}
	store, selectDecl := decl("store", 3), decl("select", 2)
	variables := make([]ast.ASTHandle, 3)
	for i := range variables {
		variables[i], err = m.MkVar(uint32(2-i), integer)
		patternMust(t, err)
	}
	stored, err := m.MkApp(store, variables)
	patternMust(t, err)
	selected, err := m.MkApp(selectDecl, []ast.ASTHandle{stored, variables[1]})
	patternMust(t, err)
	body, err := m.MkEq(selected, variables[2])
	patternMust(t, err)
	names := []symbol.Symbol{symbol.Numerical(1), symbol.Numerical(2), symbol.Numerical(3)}
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer, integer, integer}, Names: names, Body: body})
	patternMust(t, err)
	patternMust(t, m.IncRef(q))
	t.Cleanup(func() { patternMust(t, m.DecRef(q)) })
	matcher, err := newExprPatternMatcher(m)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, matcher.Close()) })
	patternMust(t, matcher.InitializeDatabase())
	if len(matcher.precompiled) != 46 {
		t.Fatal("embedded assertion count", len(matcher.precompiled))
	}
	patterns, weight, matched, err := matcher.MatchQuantifier(q)
	patternMust(t, err)
	if !matched || weight != 0 || len(patterns) != 1 {
		t.Fatal("database match", matched, weight, len(patterns))
	}
	defer func() { patternMust(t, m.DelRef(&patterns[0])) }()
	p, err := m.RefGet(patterns[0])
	patternMust(t, err)
	term, err := m.GetArg(p, 0)
	patternMust(t, err)
	if term != stored {
		t.Fatal("database-instantiated store pattern", term, stored)
	}
}

func TestExprPatternMatcherAssociativeCommutativeBacktracking(t *testing.T) {
	m := patternManager(t)
	b, err := m.MkBoolSort()
	patternMust(t, err)
	decl := func(name string) ast.ASTHandle {
		t.Helper()
		s, e := m.Symbols.Intern(name)
		patternMust(t, e)
		d, e := m.MkFuncDecl(s, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
		patternMust(t, e)
		patternMust(t, m.IncRef(d))
		t.Cleanup(func() { patternMust(t, m.DecRef(d)) })
		return d
	}
	patternF, patternG, actualF, actualG := decl("?f"), decl("?g"), decl("f"), decl("g")
	vars := make([]ast.ASTHandle, 3)
	for i := range vars {
		vars[i], err = m.MkVar(uint32(2-i), b)
		patternMust(t, err)
	}
	app := func(d ast.ASTHandle, v ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(d, []ast.ASTHandle{v})
		patternMust(t, e)
		return h
	}
	pf0, pf1, pg2 := app(patternF, vars[0]), app(patternF, vars[1]), app(patternG, vars[2])
	templateBody, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpOr, nil, []ast.ASTHandle{pf0, pf1, pg2})
	patternMust(t, err)
	annotation, err := m.MkPattern([]ast.ASTHandle{pf0})
	patternMust(t, err)
	names := []symbol.Symbol{symbol.Numerical(1), symbol.Numerical(2), symbol.Numerical(3)}
	template, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b, b, b}, Names: names, Body: templateBody, Patterns: []ast.ASTHandle{annotation}})
	patternMust(t, err)
	actualBody, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpOr, nil, []ast.ASTHandle{app(actualG, vars[2]), app(actualF, vars[1]), app(actualF, vars[0])})
	patternMust(t, err)
	actual, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b, b, b}, Names: names, Body: actualBody})
	patternMust(t, err)
	for _, h := range []ast.ASTHandle{template, actual} {
		patternMust(t, m.IncRef(h))
		h := h
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
	}
	matcher, err := newExprPatternMatcher(m)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, matcher.Close()) })
	patternMust(t, matcher.Compile(template))
	patterns, _, matched, err := matcher.MatchQuantifier(actual)
	patternMust(t, err)
	if !matched || len(patterns) != 1 {
		t.Fatal("AC permutation match", matched, len(patterns))
	}
	patternMust(t, m.DelRef(&patterns[0]))
}

// expr_pattern_match::instantiate walks the template with for_each_ast,
// whose LIFO stack instantiates the last argument first: (f (g x0) (h x1))
// builds (h b) before (g a).
func TestExprPatternMatcherInstantiatesLastArgumentFirst(t *testing.T) {
	m := patternManager(t)
	b, err := m.MkBoolSort()
	patternMust(t, err)
	decl := func(name string, arity int) ast.ASTHandle {
		t.Helper()
		s, e := m.Symbols.Intern(name)
		patternMust(t, e)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = b
		}
		d, e := m.MkFuncDecl(s, domain, b, ast.ASTFuncDeclInfo{})
		patternMust(t, e)
		patternMust(t, m.IncRef(d))
		t.Cleanup(func() { patternMust(t, m.DecRef(d)) })
		return d
	}
	f, g, h := decl("order-f", 2), decl("order-g", 1), decl("order-h", 1)
	constant := func(name string) ast.ASTHandle {
		t.Helper()
		s, e := m.Symbols.Intern(name)
		patternMust(t, e)
		c, e := m.MkConst(s, b)
		patternMust(t, e)
		patternMust(t, m.IncRef(c))
		t.Cleanup(func() { patternMust(t, m.DecRef(c)) })
		return c
	}
	a, c := constant("order-a"), constant("order-c")
	x0, err := m.MkVar(0, b)
	patternMust(t, err)
	x1, err := m.MkVar(1, b)
	patternMust(t, err)
	gx, err := m.MkApp(g, []ast.ASTHandle{x0})
	patternMust(t, err)
	hx, err := m.MkApp(h, []ast.ASTHandle{x1})
	patternMust(t, err)
	template, err := m.MkApp(f, []ast.ASTHandle{gx, hx})
	patternMust(t, err)
	patternMust(t, m.IncRef(template))
	t.Cleanup(func() { patternMust(t, m.DecRef(template)) })
	matcher, err := newExprPatternMatcher(m)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, matcher.Close()) })
	result, err := matcher.instantiate(template, map[ast.ASTHandle]uint32{}, map[ast.ASTHandle]ast.ASTHandle{x0: a, x1: c}, map[ast.ASTHandle]ast.ASTHandle{})
	patternMust(t, err)
	patternMust(t, m.IncRef(result))
	t.Cleanup(func() { patternMust(t, m.DecRef(result)) })
	ga, err := m.GetArg(result, 0)
	patternMust(t, err)
	hc, err := m.GetArg(result, 1)
	patternMust(t, err)
	gID, err := m.GetASTID(ga)
	patternMust(t, err)
	hID, err := m.GetASTID(hc)
	patternMust(t, err)
	if hID >= gID {
		t.Fatal("the last argument must be instantiated first", hID, gID)
	}
}
