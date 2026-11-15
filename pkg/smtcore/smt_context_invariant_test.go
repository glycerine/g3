package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextInvariantWatchesAssignmentsAndJustifications(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "context-invariant-a")
	b := smtRelevancyBool(t, m, "context-invariant-b")
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.Internalize(a, true))
	astTestMust(t, state.Internalize(b, true))
	aLiteral, err := state.Literal(a)
	astTestMust(t, err)
	bLiteral, err := state.Literal(b)
	astTestMust(t, err)
	astTestMust(t, state.addClause([]SMTLiteral{aLiteral.Negated(), bLiteral}, false, ast.ASTHandle{}))
	astTestMust(t, state.AssignLiteral(aLiteral, NewSMTBooleanAxiom()))
	consistent, err := state.Propagate()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("unexpected Boolean conflict")
	}
	astTestMust(t, state.CheckInvariant())
	astTestMust(t, state.ValidateJustification(bLiteral.Var()))

	lastAssignment := state.assignment[len(state.assignment)-1]
	state.assignment = state.assignment[:len(state.assignment)-1]
	if err = state.CheckInvariant(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("assignment vector corruption", err)
	}
	state.assignment = append(state.assignment, lastAssignment)
	astTestMust(t, state.CheckInvariant())

	clause := state.testClauses()[0].clause
	watch := &state.watches[clause.Literals()[0].Negated().Index()]
	if !watch.RemoveClause(clause) {
		t.Fatal("missing test watch")
	}
	if err = state.CheckInvariant(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("watch corruption", err)
	}
	astTestMust(t, watch.InsertClause(clause))
	astTestMust(t, state.CheckInvariant())
}

func TestSMTContextInvariantDetectsInvalidClauseJustification(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "context-justification-a")
	b := smtRelevancyBool(t, m, "context-justification-b")
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.Internalize(a, true))
	astTestMust(t, state.Internalize(b, true))
	aLiteral, err := state.Literal(a)
	astTestMust(t, err)
	bLiteral, err := state.Literal(b)
	astTestMust(t, err)
	astTestMust(t, state.addClause([]SMTLiteral{aLiteral, bLiteral}, false, ast.ASTHandle{}))
	state.booleanData[aLiteral.Var()].SetJustification(NewSMTBooleanClause(state.testClauses()[0].clause))
	if err = state.ValidateJustification(aLiteral.Var()); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("false clause justification", err)
	}
}
