package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// smtWMaxFixture registers theory_wmaxsat with one slack variable per weight.
func smtWMaxFixture(t *testing.T, weights ...int64) (*ast.ASTManager, *SMTInternalizerState, *SMTWeightedMaxTheory) {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	filter, err := tactic.NewFilterModel(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, filter.Close()) })
	theory, err := solver.EnsureWeightedMaxTheory(filter)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	for i, weight := range weights {
		formula, err := m.MkConst(astTestSymbol(t, m, fmt.Sprint("wmax-soft-", i)), boolean)
		astTestMust(t, err)
		astTestPin(t, m, formula)
		slack, assertion, err := theory.AddWeighted(formula, big.NewRat(weight, 1))
		astTestMust(t, err)
		astTestPin(t, m, slack)
		astTestPin(t, m, assertion)
	}
	return m, solver.kernel.internalizer, theory
}

func smtWMaxAssignTrue(t *testing.T, state *SMTInternalizerState, theory *SMTWeightedMaxTheory, index int) {
	t.Helper()
	// Relevant atoms reach assign_eh through propagate_atoms.
	astTestMust(t, state.markRelevant(theory.variables[index].expr))
	astTestMust(t, state.AssignLiteral(NewSMTLiteral(theory.variables[index].boolean, false), NewSMTBooleanAxiom()))
	_, err := state.PropagateCore()
	astTestMust(t, err)
}

// AUDIT.md smtcore_13-F8: compare_cost sorts the asserted slack variables by
// decreasing weight, so block() and mk_block() take the heaviest first.
func TestSMTWeightedMaxBlocksHeaviestCostsFirst(t *testing.T) {
	_, state, theory := smtWMaxFixture(t, 1, 5, 2)
	astTestMust(t, theory.InitMinCost(big.NewRat(5, 1)))
	astTestMust(t, state.PushScope())
	smtWMaxAssignTrue(t, state, theory, 0)
	if state.Inconsistent() {
		t.Fatal("cost 1 is below the minimum cost 5")
	}
	smtWMaxAssignTrue(t, state, theory, 1)
	if !state.Inconsistent() {
		t.Fatal("cost 6 did not block")
	}
	conflict, _ := state.Conflict()
	owner, _ := conflict.Justification()
	justification, ok := owner.(*SMTTheoryConflictJustification)
	if !ok || len(justification.Literals) != 1 || justification.Literals[0] != NewSMTLiteral(theory.variables[1].boolean, false) {
		t.Fatalf("block() conflict should be the weight-5 slack alone: %#v", owner)
	}
	astTestMust(t, state.PopScopes(1))

	// mk_block over all three asserted slack variables.
	astTestMust(t, theory.InitMinCost(big.NewRat(100, 1)))
	astTestMust(t, state.PushScope())
	for i := range theory.variables {
		smtWMaxAssignTrue(t, state, theory, i)
	}
	if state.Inconsistent() || len(theory.costs) != 3 {
		t.Fatal("assignments below the minimum cost", len(theory.costs))
	}
	astTestMust(t, theory.InitMinCost(big.NewRat(5, 1)))
	block, err := theory.MakeBlock()
	astTestMust(t, err)
	want, err := state.manager.MkNot(theory.variables[1].expr)
	astTestMust(t, err)
	astTestPin(t, state.manager, block)
	count, err := state.manager.GetNumArgs(block)
	astTestMust(t, err)
	first, err := state.manager.GetArg(block, 0)
	astTestMust(t, err)
	if count != 1 || first != want {
		t.Fatal("mk_block should be (or (not w_b))", count, first, want)
	}
}

// AUDIT.md smtcore_13-F10: final_check_eh only normalizes; assign_eh skips
// variables that are already assigned or disabled, and its cost is restored
// by the trail.
func TestSMTWeightedMaxFinalCheckDoesNotBlock(t *testing.T) {
	_, state, theory := smtWMaxFixture(t, 5)
	astTestMust(t, theory.InitMinCost(big.NewRat(5, 1)))
	astTestMust(t, theory.DisableVariable(theory.variables[0].expr))
	astTestMust(t, state.PushScope())
	smtWMaxAssignTrue(t, state, theory, 0)
	status, err := theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone || state.Inconsistent() {
		t.Fatal("disabled slack variable raised a conflict", status)
	}
	if cost, _ := theory.Cost(); cost.Sign() != 0 {
		t.Fatal("disabled slack variable was counted", cost)
	}
	astTestMust(t, state.PopScopes(1))
}

func TestSMTWeightedMaxCostTrailRestore(t *testing.T) {
	_, state, theory := smtWMaxFixture(t, 2, 3)
	astTestMust(t, theory.InitMinCost(big.NewRat(10, 1)))
	astTestMust(t, state.PushScope())
	smtWMaxAssignTrue(t, state, theory, 0)
	astTestMust(t, state.PushScope())
	smtWMaxAssignTrue(t, state, theory, 1)
	// A repeated assign_eh is ignored once m_assigned[tv] is set.
	astTestMust(t, theory.Assign(theory.variables[1].boolean, true))
	if cost, _ := theory.Cost(); cost.Cmp(big.NewRat(5, 1)) != 0 || len(theory.costs) != 2 {
		t.Fatal("cost after two assignments", cost, theory.costs)
	}
	status, err := theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone || state.Inconsistent() {
		t.Fatal("final check below the minimum cost")
	}
	astTestMust(t, state.PopScopes(1))
	if cost, _ := theory.Cost(); cost.Cmp(big.NewRat(2, 1)) != 0 || len(theory.costs) != 1 || theory.variables[1].assigned {
		t.Fatal("pop did not restore the cost trail", cost, theory.costs)
	}
	astTestMust(t, state.PopScopes(1))
	if cost, _ := theory.Cost(); cost.Sign() != 0 || len(theory.costs) != 0 || theory.variables[0].assigned {
		t.Fatal("pop did not restore the cost trail", cost, theory.costs)
	}
}
