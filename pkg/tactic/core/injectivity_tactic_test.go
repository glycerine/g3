package core

// Source: src/tactic/core/injectivity_tactic.cpp end-to-end inverse discovery
// and equality rewriting.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type injectivityFixture struct {
	m        *ast.ASTManager
	runner   *InjectivityTactic
	sort     ast.ASTHandle
	f, g     ast.ASTHandle
	axiom    ast.ASTHandle
	variable ast.ASTHandle
}

func newInjectivityFixture(t *testing.T) injectivityFixture {
	t.Helper()
	m, c, globals := simplifyFixture(t)
	simplifyMust(t, m.RegisterDeclarationPlugins())
	runner, err := NewInjectivityTactic(m, c, globals, nil)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	sortName, err := m.Symbols.Intern("InjU")
	simplifyMust(t, err)
	sort, err := m.MkSort(sortName, ast.ASTSortInfo{})
	simplifyMust(t, err)
	nameF, err := m.Symbols.Intern("inj-f")
	simplifyMust(t, err)
	nameG, err := m.Symbols.Intern("inj-g")
	simplifyMust(t, err)
	f, err := m.MkFuncDecl(nameF, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	g, err := m.MkFuncDecl(nameG, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	simplifyPin(t, m, f)
	simplifyPin(t, m, g)
	v, err := m.MkVar(0, sort)
	simplifyMust(t, err)
	fv, err := m.MkApp(f, []ast.ASTHandle{v})
	simplifyMust(t, err)
	gfv, err := m.MkApp(g, []ast.ASTHandle{fv})
	simplifyMust(t, err)
	eq, err := m.MkEq(gfv, v)
	simplifyMust(t, err)
	name, err := m.Symbols.Intern("x")
	simplifyMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{name}, Body: eq})
	simplifyMust(t, err)
	simplifyPin(t, m, q)
	return injectivityFixture{m: m, runner: runner, sort: sort, f: f, g: g, axiom: q, variable: v}
}

func (f injectivityFixture) constant(t *testing.T, name string) ast.ASTHandle {
	t.Helper()
	s, err := f.m.Symbols.Intern(name)
	simplifyMust(t, err)
	h, err := f.m.MkConst(s, f.sort)
	simplifyMust(t, err)
	simplifyPin(t, f.m, h)
	return h
}

func (f injectivityFixture) goal(t *testing.T, proofs, cores bool, forms ...ast.ASTHandle) *tactic.Goal {
	t.Helper()
	g, err := tactic.NewGoal(f.m, proofs, false, cores)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, g.Close()) })
	for _, form := range forms {
		if proofs {
			pr, err := f.m.MkProofAsserted(form)
			simplifyMust(t, err)
			simplifyMust(t, g.AssertWithProof(form, pr, ast.ASTDependency{}))
		} else {
			simplifyMust(t, g.Assert(form))
		}
	}
	return g
}

func TestInjectivityDiscoversInverseAndRewritesEquality(t *testing.T) {
	f := newInjectivityFixture(t)
	a, b := f.constant(t, "inj-a"), f.constant(t, "inj-b")
	fa, err := f.m.MkApp(f.f, []ast.ASTHandle{a})
	simplifyMust(t, err)
	fb, err := f.m.MkApp(f.f, []ast.ASTHandle{b})
	simplifyMust(t, err)
	equality, err := f.m.MkEq(fa, fb)
	simplifyMust(t, err)
	goal := f.goal(t, false, false, f.axiom, equality)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, f.runner.Apply(goal, &result))
	got, err := goal.Form(1)
	simplifyMust(t, err)
	want, err := f.m.MkEq(a, b)
	simplifyMust(t, err)
	if got != want || len(result.Goals) != 1 || result.Goals[0] != goal {
		t.Fatal("injective equality rewrite", got, want)
	}
}

// AUDIT.md tactic_core_01-F2: injectivity_tactic::operator() never resets
// m_eq, so a shared equality cached by an earlier application (when the
// inverse was unknown) stays unrewritten in a later goal.
func TestInjectivityCacheSurvivesApplications(t *testing.T) {
	f := newInjectivityFixture(t)
	a, b := f.constant(t, "inj-cache-a"), f.constant(t, "inj-cache-b")
	fa, err := f.m.MkApp(f.f, []ast.ASTHandle{a})
	simplifyMust(t, err)
	fb, err := f.m.MkApp(f.f, []ast.ASTHandle{b})
	simplifyMust(t, err)
	equality, err := f.m.MkEq(fa, fb)
	simplifyMust(t, err)
	simplifyPin(t, f.m, equality)
	boolSort, err := f.m.MkBoolSort()
	simplifyMust(t, err)
	clauses := make([]ast.ASTHandle, 2)
	for i, name := range []string{"inj-cache-q", "inj-cache-r"} {
		s, err := f.m.Symbols.Intern(name)
		simplifyMust(t, err)
		atom, err := f.m.MkConst(s, boolSort)
		simplifyMust(t, err)
		clauses[i], err = f.m.MkOr(equality, atom)
		simplifyMust(t, err)
		simplifyPin(t, f.m, clauses[i])
	}
	first := f.goal(t, false, false, clauses...)
	var firstResult tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, firstResult.Close()) })
	simplifyMust(t, f.runner.Apply(first, &firstResult))
	second := f.goal(t, false, false, f.axiom, clauses[0], clauses[1])
	var secondResult tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, secondResult.Close()) })
	simplifyMust(t, f.runner.Apply(second, &secondResult))
	for i := uint32(1); i < 3; i++ {
		got, err := second.Form(i)
		simplifyMust(t, err)
		if got != clauses[i-1] {
			t.Fatal("cached equality was rewritten in a later goal", i, got, clauses[i-1])
		}
	}
	simplifyMust(t, f.runner.Cleanup())
	third := f.goal(t, false, false, f.axiom, clauses[0], clauses[1])
	var thirdResult tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, thirdResult.Close()) })
	simplifyMust(t, f.runner.Apply(third, &thirdResult))
	got, err := third.Form(1)
	simplifyMust(t, err)
	if got == clauses[0] {
		t.Fatal("cleanup did not drop the rewriter cache")
	}
}
