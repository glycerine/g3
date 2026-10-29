package core

// Source: src/tactic/core/injectivity_tactic.cpp persistent InjHelper,
// cleanup replacement, parameter update, and translation.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestInjectivityRelationsPersistUntilCleanupAndTranslationStartsEmpty(t *testing.T) {
	f := newInjectivityFixture(t)
	first := f.goal(t, false, false, f.axiom)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, f.runner.Apply(first, &result))
	a, b := f.constant(t, "persist-a"), f.constant(t, "persist-b")
	fa, err := f.m.MkApp(f.f, []ast.ASTHandle{a})
	simplifyMust(t, err)
	fb, err := f.m.MkApp(f.f, []ast.ASTHandle{b})
	simplifyMust(t, err)
	eq, err := f.m.MkEq(fa, fb)
	simplifyMust(t, err)
	simplifyPin(t, f.m, eq)
	want, err := f.m.MkEq(a, b)
	simplifyMust(t, err)
	second := f.goal(t, false, false, eq)
	simplifyMust(t, f.runner.Apply(second, &result))
	got, err := second.Form(0)
	simplifyMust(t, err)
	if got != want {
		t.Fatal("discovered relation did not persist")
	}
	simplifyMust(t, f.runner.Cleanup())
	third := f.goal(t, false, false, eq)
	simplifyMust(t, f.runner.Apply(third, &result))
	got, err = third.Form(0)
	simplifyMust(t, err)
	if got != eq {
		t.Fatal("cleanup retained injectivity relation")
	}
	copy, err := f.runner.Translate(f.m)
	simplifyMust(t, err)
	translated := copy.(*InjectivityTactic)
	simplifyMust(t, translated.Close())
}
