package opt_test

// Source: src/opt/opt_context.cpp lex MaxSMT lower/upper publication.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestLexSoftResultsPublishTightBoundsAndAssignments(t *testing.T) {
	manager, evaluator := priorityFixture(t)
	p := priorityBool(t, manager, "bounds-p")
	q := priorityBool(t, manager, "bounds-q")
	both, err := manager.MkAnd(p, q)
	if err != nil {
		t.Fatal(err)
	}
	mutex, err := manager.MkNot(both)
	if err != nil {
		t.Fatal(err)
	}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.Assert(mutex); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{p}, []numeral.Rational{testRat(2, 1)}); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{q}, []numeral.Rational{testRat(3, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	for i := 0; i < 2; i++ {
		lower, upper, err := solver.SoftBounds(result, i)
		if err != nil || !ratEq(&lower, &upper) || !ratEq(&lower, &result.Values[i].SoftCost) {
			t.Fatal("soft bounds", i, lower, upper, result.Values[i], err)
		}
	}
}
