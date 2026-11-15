package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
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

// AUDIT.md tactic_core_02-F2: pure_args strips one not and then accepts an
// uninterpreted constant, true or false, so (or a (not false) b) is in m_ge.
func TestPBPreprocessNegatedBooleanConstantIsPure(t *testing.T) {
	f := newPBPreprocessFixture(t)
	a, b := f.vars[0], f.vars[1]
	no, e := f.m.MkFalse()
	simplifyMust(t, e)
	notFalse, e := f.m.MkNot(no)
	simplifyMust(t, e)
	first, e := f.m.MkOrArgs([]ast.ASTHandle{a, notFalse, b})
	simplifyMust(t, e)
	notA, e := f.m.MkNot(a)
	simplifyMust(t, e)
	second, e := f.m.MkOrArgs([]ast.ASTHandle{notA, b})
	simplifyMust(t, e)
	g := f.goal(first, second)
	f.apply(g)
	if n, e := g.Size(); e != nil || n != 0 {
		t.Fatal("(not false) clause was not treated as pure", n, e)
	}
}

// AUDIT.md tactic_core_02-F3: the declassifier uses for_each_expr, which
// visits quantifier patterns, so a constant that occurs only in a pattern
// of an m_other formula is declassified.
func TestPBPreprocessPatternOccurrenceDeclassifies(t *testing.T) {
	f := newPBPreprocessFixture(t)
	a := f.vars[0]
	b, e := f.m.MkBoolSort()
	simplifyMust(t, e)
	fn, e := f.m.MkFuncDecl(pbPreprocessSymbol(t, f.m, "pb-pattern-f"), []ast.ASTHandle{b, b}, b, ast.ASTFuncDeclInfo{})
	simplifyMust(t, e)
	x, e := f.m.MkVar(0, b)
	simplifyMust(t, e)
	trigger, e := f.m.MkApp(fn, []ast.ASTHandle{x, a})
	simplifyMust(t, e)
	pattern, e := f.m.MkPattern([]ast.ASTHandle{trigger})
	simplifyMust(t, e)
	notX, e := f.m.MkNot(x)
	simplifyMust(t, e)
	body, e := f.m.MkOr(x, notX)
	simplifyMust(t, e)
	q, e := f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{pbPreprocessSymbol(t, f.m, "x")}, Body: body, Patterns: []ast.ASTHandle{pattern}})
	simplifyMust(t, e)
	g := f.goal(a, q)
	f.apply(g)
	if n, e := g.Size(); e != nil || n != 2 {
		t.Fatal("constant used in a pattern was eliminated", n, e)
	}
}
