package core

// Source: src/tactic/core/injectivity_tactic.cpp finder::is_axiom guards.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestInjectivityFinderOrientationAndExactDeBruijnShape(t *testing.T) {
	f := newInjectivityFixture(t)
	foundF, foundG, matched, err := f.runner.inverseAxiom(f.axiom)
	simplifyMust(t, err)
	if !matched || foundF != f.f || foundG != f.g {
		kind, _ := f.m.GetKind(f.axiom)
		forall, _ := f.m.IsForall(f.axiom)
		decls, _ := f.m.GetNumDeclarations(f.axiom)
		body, _ := f.m.GetQuantifierBody(f.axiom)
		left, right, eq, ee := f.runner.equality(body)
		t.Fatal("canonical inverse axiom", matched, foundF, foundG, "q", kind, forall, decls, "eq", eq, ee, left, right)
	}
	body, err := f.m.GetQuantifierBody(f.axiom)
	simplifyMust(t, err)
	left, right, matchedEq, err := f.runner.equality(body)
	simplifyMust(t, err)
	if !matchedEq {
		t.Fatal("fixture equality")
	}
	reversed, err := f.m.MkEq(right, left)
	simplifyMust(t, err)
	name, err := f.m.Symbols.Intern("reversed")
	simplifyMust(t, err)
	q, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.sort}, Names: []usymbol.Symbol{name}, Body: reversed})
	simplifyMust(t, err)
	_, _, matched, err = f.runner.inverseAxiom(q)
	simplifyMust(t, err)
	if !matched {
		t.Fatal("source accepts equality in either orientation")
	}
	wrong, err := f.m.MkVar(1, f.sort)
	simplifyMust(t, err)
	fw, err := f.m.MkApp(f.f, []ast.ASTHandle{wrong})
	simplifyMust(t, err)
	gfw, err := f.m.MkApp(f.g, []ast.ASTHandle{fw})
	simplifyMust(t, err)
	badBody, err := f.m.MkEq(gfw, wrong)
	simplifyMust(t, err)
	bad, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.sort}, Names: []usymbol.Symbol{name}, Body: badBody})
	simplifyMust(t, err)
	_, _, matched, err = f.runner.inverseAxiom(bad)
	simplifyMust(t, err)
	if matched {
		t.Fatal("nonzero de Bruijn index accepted")
	}
}
