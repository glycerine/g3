package core

// Source: src/tactic/core/injectivity_tactic.cpp recursive rewriter_eq use.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestInjectivityRewritesNestedEqualityOnlyForMarkedDeclaration(t *testing.T) {
	f := newInjectivityFixture(t)
	a, b := f.constant(t, "nested-a"), f.constant(t, "nested-b")
	fa, err := f.m.MkApp(f.f, []ast.ASTHandle{a})
	simplifyMust(t, err)
	fb, err := f.m.MkApp(f.f, []ast.ASTHandle{b})
	simplifyMust(t, err)
	eq, err := f.m.MkEq(fa, fb)
	simplifyMust(t, err)
	nested, err := f.m.MkNot(eq)
	simplifyMust(t, err)
	goal := f.goal(t, false, false, f.axiom, nested)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, f.runner.Apply(goal, &result))
	wantEq, err := f.m.MkEq(a, b)
	simplifyMust(t, err)
	want, err := f.m.MkNot(wantEq)
	simplifyMust(t, err)
	got, err := goal.Form(1)
	simplifyMust(t, err)
	if got != want {
		t.Fatal("nested rewrite", got, want)
	}
}
