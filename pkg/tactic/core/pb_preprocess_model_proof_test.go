package core

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestPBPreprocessReconstructsEliminatedBooleanModel(t *testing.T) {
	f := newPBPreprocessFixture(t)
	declarations := make([]ast.ASTHandle, 0, 2)
	for _, term := range f.vars[:2] {
		declaration, err := f.m.GetDecl(term)
		simplifyMust(t, err)
		simplifyPin(t, f.m, declaration)
		declarations = append(declarations, declaration)
	}
	goal := f.goal(f.ge(f.vars[:2], []string{"1", "1"}, "1"))
	result := f.apply(goal)
	if result.ModelConverter == nil {
		t.Fatal("one-sided elimination omitted its model converter")
	}
	var converted tactic.ModelRef
	t.Cleanup(func() { simplifyMust(t, converted.Close()) })
	simplifyMust(t, tactic.ModelConverterToModel(f.m, result.ModelConverter, &converted))
	model, err := converted.Get()
	simplifyMust(t, err)
	for _, declaration := range declarations {
		value, err := model.ConstantInterpretation(declaration)
		simplifyMust(t, err)
		if !f.m.IsTrue(value) {
			t.Fatalf("eliminated declaration %v reconstructed as %v, want true", declaration, value)
		}
	}
}

func TestPBPreprocessRejectsProofGoalsBeforeMutation(t *testing.T) {
	f := newPBPreprocessFixture(t)
	simplifyMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
	formula := f.ge(f.vars[:1], []string{"1"}, "1")
	goal, err := tactic.NewGoal(f.m, true, true, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(formula))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	err = f.runner.Apply(goal, &result)
	if err == nil || !strings.Contains(err.Error(), "does not support proof") {
		t.Fatalf("proof goal error = %v", err)
	}
	if size, sizeErr := goal.Size(); sizeErr != nil || size != 1 {
		t.Fatalf("proof refusal mutated goal: size=%d err=%v", size, sizeErr)
	}
}
