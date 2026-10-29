package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func apiTacticModelResult(t *testing.T, f *smtAPIFixture, declaration, definition ast.ASTHandle) *APITacticResult {
	t.Helper()
	config, err := f.c.command.ModelEvaluatorConfig()
	astTestMust(t, err)
	converter, err := tactic.NewExtensionModel(f.m, config)
	astTestMust(t, err)
	astTestMust(t, converter.Insert(declaration, definition))
	astTestMust(t, converter.IncRef())
	goal, err := f.c.NewGoal(true, false, false)
	astTestMust(t, err)
	copy, err := tactic.NewGoalCopy(goal.goal)
	astTestMust(t, err)
	astTestMust(t, goal.Close())
	return &APITacticResult{owner: f.c.smtContextState, manager: f.m, goals: []*tactic.Goal{copy}, modelConverter: converter}
}

func TestAPITacticResultConvertModelAppliesRetainedConverter(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	constant := f.pin(f.c.Constant("api-result-converted", boolean))
	declaration, err := f.m.GetDecl(constant)
	astTestMust(t, err)
	yes := f.pin(f.c.Bool(true))
	source, err := f.c.NewAPIModel()
	astTestMust(t, err)
	result := apiTacticModelResult(t, f, declaration, yes)
	converted, err := result.ConvertModel(0, source)
	astTestMust(t, err)
	value, err := converted.ConstantInterpretation(declaration)
	astTestMust(t, err)
	if value != yes {
		t.Fatal("converter definition", value, yes)
	}
	if value, e := source.ConstantInterpretation(declaration); e != nil || !value.IsNull() {
		t.Fatal("source model mutated", value, e)
	}
	astTestMust(t, converted.Close())
	astTestMust(t, source.Close())
	astTestMust(t, result.Close())
}
