package opt_test

// Source: src/opt/maxsmt.cpp commit_assignment across lexicographic objectives.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestMaxSMTLexCommitPreservesSoftAssignmentsAndBounds(t *testing.T) {
	manager, evaluator := sortMaxFixture(t)
	p := sortMaxBool(t, manager, "maxsmt-lex-p")
	if err := manager.IncRef(p); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(p)
	q := sortMaxBool(t, manager, "maxsmt-lex-q")
	if err := manager.IncRef(q); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(q)
	both, err := manager.MkAnd(p, q)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.IncRef(both); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(both)
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
	if err = solver.AddSoftObjective([]ast.ASTHandle{p}, []*big.Rat{big.NewRat(1, 2)}); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{q}, []*big.Rat{big.NewRat(1, 2)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	first, err := solver.SoftAssignment(result, 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := solver.SoftAssignment(result, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || !first[0] || len(second) != 1 || second[0] {
		t.Fatal("lex soft assignments", first, second)
	}
	if result.Values[0].SoftLower.Cmp(&result.Values[0].SoftUpper) != 0 || result.Values[1].SoftLower.Cmp(&result.Values[1].SoftUpper) != 0 {
		t.Fatal("optimal soft bounds", result.Values)
	}
	first[0] = false
	again, err := solver.SoftAssignment(result, 0)
	if err != nil || !again[0] {
		t.Fatal("assignment API alias", again, err)
	}
}
