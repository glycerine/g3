package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestDomSimplifyDefaultCallbackSubstitutesBooleanIff(t *testing.T) {
	f := newDomFixture(t)
	iff, err := f.m.MkIff(f.a, f.b)
	domMust(t, err)
	s, err := newExprSubstitutionSimplifier(f.m)
	domMust(t, err)
	t.Cleanup(func() { domMust(t, s.Close()) })
	ok, err := s.AssertExpression(iff, false)
	domMust(t, err)
	if !ok || s.ScopeLevel() != 1 {
		t.Fatal("assert_expr pushes one scope and succeeds", ok, s.ScopeLevel())
	}
	ref, err := f.m.NewRef(f.b)
	domMust(t, err)
	t.Cleanup(func() { domMust(t, f.m.DelRef(&ref)) })
	domMust(t, s.Rewrite(ref))
	got, err := f.m.RefGet(ref)
	domMust(t, err)
	if got != f.a {
		t.Fatalf("Boolean iff substitution = %v, want %v", got, f.a)
	}
	domMust(t, s.Pop(1))
}

func TestDomSimplifyDoesNotPropagateGoalEqualitiesWithProofs(t *testing.T) {
	f := newDomFixture(t)
	eq, err := f.m.MkEq(f.a, f.b)
	domMust(t, err)
	g := f.goal(t, true, eq, eq)
	f.run(t, g)
	second, err := g.Form(1)
	domMust(t, err)
	if second != eq {
		t.Fatalf("proof goal propagated an equality between formulas: got %v, want %v", second, eq)
	}
}

func TestDomSimplifyDoesNotPropagateGoalEqualitiesWithDependencies(t *testing.T) {
	f := newDomFixture(t)
	eq, err := f.m.MkEq(f.a, f.b)
	domMust(t, err)
	dep, err := f.m.DependencyLeaf(f.c)
	domMust(t, err)
	domMust(t, f.m.DependencyInc(dep))
	t.Cleanup(func() { domMust(t, f.m.DependencyDec(dep)) })
	g, err := tactic.NewGoal(f.m, false, false, true)
	domMust(t, err)
	t.Cleanup(func() { domMust(t, g.Close()) })
	if err = g.AssertWithProof(eq, ast.ASTHandle{}, dep); err != nil {
		t.Fatal(err)
	}
	if err = g.AssertWithProof(eq, ast.ASTHandle{}, dep); err != nil {
		t.Fatal(err)
	}
	firstDep, err := g.Dependency(0)
	domMust(t, err)
	secondDep, err := g.Dependency(1)
	domMust(t, err)
	if firstDep.IsNull() || secondDep.IsNull() {
		t.Fatal("test setup lost goal dependency", firstDep, secondDep)
	}
	var out tactic.TacticResult
	t.Cleanup(func() { domMust(t, out.Close()) })
	runner, err := NewDomSimplifyTactic(f.m, nil)
	domMust(t, err)
	t.Cleanup(func() { domMust(t, runner.Close()) })
	domMust(t, runner.Apply(g, &out))
	second, err := g.Form(1)
	domMust(t, err)
	if second != eq {
		t.Fatalf("dependency goal propagated an equality between formulas: got %v, want %v", second, eq)
	}
}
