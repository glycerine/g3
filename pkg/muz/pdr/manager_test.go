package pdr_test

// Source: src/muz/pdr/pdr_manager.cpp state creation, formula conversion,
// atom recognition, background assertion and implication checks.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func pdrManagerFixture(t *testing.T, maximum uint32) (*cmd_context.Context, *ast.ASTManager, *pdr.Manager) {
	t.Helper()
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	manager, err := command.Manager()
	pdrMust(t, err)
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	owner, err := pdr.NewManager(manager, nil, maximum, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	pdrMust(t, err)
	return command, manager, owner
}

func TestManagerStateCreationConversionAndAtomRecognition(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	prefix := pdrFunction(t, manager, "manager_state", nil, boolean)
	newDeclaration, err := owner.NewPredicate(prefix)
	pdrMust(t, err)
	oldDeclaration, err := owner.OldPredicate(prefix, 0)
	pdrMust(t, err)
	oldTwoDeclaration, err := owner.OldPredicate(prefix, 2)
	pdrMust(t, err)
	if newDeclaration == oldDeclaration || oldDeclaration == oldTwoDeclaration {
		t.Fatal("state versions were not distinct")
	}
	if converted, x := owner.NewToOld(newDeclaration, 2); x != nil || converted != oldTwoDeclaration {
		t.Fatal("new-to-old declaration conversion", converted, x)
	}
	if converted, x := owner.OldToNew(oldDeclaration, 0); x != nil || converted != newDeclaration {
		t.Fatal("old-to-new declaration conversion", converted, x)
	}
	oldZero, err := owner.OldZeroPredicates()
	pdrMust(t, err)
	if len(oldZero) != 1 || oldZero[0] != oldDeclaration {
		t.Fatal("old-zero declarations", oldZero)
	}

	newState, err := manager.MkApp(newDeclaration, nil)
	pdrMust(t, err)
	oldState, err := manager.MkApp(oldDeclaration, nil)
	pdrMust(t, err)
	convertedFormula, err := owner.FormulaNewToOld(newState, 0, true)
	pdrMust(t, err)
	if convertedFormula != oldState {
		t.Fatal("formula conversion", convertedFormula, oldState)
	}
	if old, x := owner.IsOldFormula(oldState); x != nil || !old {
		t.Fatal("old formula classification", old, x)
	}
	if newFormula, x := owner.IsNewFormula(newState); x != nil || !newFormula {
		t.Fatal("new formula classification", newFormula, x)
	}

	plain := pdrBool(t, manager, "manager_plain")
	equality, err := manager.MkEq(plain, newState)
	pdrMust(t, err)
	state, value, found, err := owner.TryStateAndValueFromAtom(equality)
	pdrMust(t, err)
	if !found || state != newState || value != plain {
		t.Fatal("reversed state equality", state, value, found)
	}
	negated, err := manager.MkNot(newState)
	pdrMust(t, err)
	state, value, found, err = owner.TryStateAndValueFromAtom(negated)
	pdrMust(t, err)
	if !found || state != newState || !manager.IsFalse(value) {
		t.Fatal("negative state atom", state, value, found)
	}
	declaration, found, err := owner.TryStateDeclarationFromAtom(newState)
	pdrMust(t, err)
	if !found || declaration != newDeclaration {
		t.Fatal("state declaration recognition", declaration, found)
	}
	if _, err = owner.OldPredicate(pdrFunction(t, manager, "bad_state", []ast.ASTHandle{boolean}, boolean), 0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("nonconstant state accepted", err)
	}

	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}

func TestManagerBooleanBackgroundSolverAndImplicationIntegration(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	p := pdrBool(t, manager, "manager_p")
	q := pdrBool(t, manager, "manager_q")
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	pdrMust(t, owner.SetBackground(p))
	solver, err := owner.NewPropSolver("manager")
	pdrMust(t, err)
	result, err := solver.CheckAssumptions([]ast.ASTHandle{notP}, false, false)
	pdrMust(t, err)
	if result.Status != lbool.False {
		t.Fatal("background was not asserted", result.Status)
	}

	if holds, x := owner.ImplicationSurelyHolds(p, p, ast.ASTHandle{}); x != nil || !holds {
		t.Fatal("reflexive implication", holds, x)
	}
	if holds, x := owner.ImplicationSurelyHolds(p, q, ast.ASTHandle{}); x != nil || holds {
		t.Fatal("invalid implication", holds, x)
	}
	conjunction, err := owner.ModelIntoCube([]ast.ASTHandle{p, q})
	pdrMust(t, err)
	lemma, err := owner.CubeIntoLemma(conjunction)
	pdrMust(t, err)
	disjuncts, err := owner.GetOr(lemma)
	pdrMust(t, err)
	if len(disjuncts) != 2 {
		t.Fatal("cube negation did not distribute over conjunction", disjuncts)
	}
	notAndRef, err := owner.NotAndRef([]ast.ASTHandle{conjunction})
	pdrMust(t, err)
	notAnd, err := manager.RefGet(notAndRef)
	pdrMust(t, err)
	disjuncts, err = owner.GetOr(notAnd)
	pdrMust(t, err)
	if len(disjuncts) != 2 {
		t.Fatal("not-and did not flatten its conjunction", disjuncts)
	}

	pdrMust(t, manager.DelRef(&notAndRef))
	if err = owner.Close(); !errors.Is(err, pdr.ErrSMTContextOwner) {
		t.Fatal("manager closed with an active proposition solver", err)
	}
	pdrMust(t, solver.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}
