package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newSMTBinaryClauseFixture(t *testing.T) (*SMTInternalizerState, SMTLiteral, SMTLiteral) {
	t.Helper()
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "binary-clause-a")
	b := smtRelevancyBool(t, m, "binary-clause-b")
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	astTestMust(t, state.SetBinaryClauseOptimization(true))
	astTestMust(t, state.Internalize(a, true))
	astTestMust(t, state.Internalize(b, true))
	aLiteral, err := state.Literal(a)
	astTestMust(t, err)
	bLiteral, err := state.Literal(b)
	astTestMust(t, err)
	return state, aLiteral, bLiteral
}

func TestSMTBinaryClauseOptimizationPropagationAndInvariant(t *testing.T) {
	state, a, b := newSMTBinaryClauseFixture(t)
	astTestMust(t, state.addClause([]SMTLiteral{a, b}, false, ast.ASTHandle{}))
	if len(state.testClauses()) != 1 || !state.testClauses()[0].Binary || state.testClauses()[0].clause != nil {
		t.Fatal("binary clause representation", state.testClauses())
	}
	if state.watches[a.Negated().Index()].FindLiteral(b) < 0 || state.watches[b.Negated().Index()].FindLiteral(a) < 0 {
		t.Fatal("reciprocal binary watches")
	}
	astTestMust(t, state.AssignLiteral(a.Negated(), NewSMTBooleanAxiom()))
	consistent, err := state.Propagate()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("binary propagation reported conflict")
	}
	if value, assignmentErr := state.Assignment(b); assignmentErr != nil || value != lbool.True {
		t.Fatal("binary implied literal", value, assignmentErr)
	}
	data, err := state.BooleanData(b.Var())
	astTestMust(t, err)
	antecedent, binary := data.Justification().Literal()
	if !binary || antecedent != a.Negated() {
		t.Fatal("binary justification", data.Justification().Kind(), antecedent)
	}
	astTestMust(t, state.CheckInvariant())
}

func TestSMTBinaryClauseOptimizationConflictAndEligibility(t *testing.T) {
	state, a, b := newSMTBinaryClauseFixture(t)
	astTestMust(t, state.addClause([]SMTLiteral{a, b}, false, ast.ASTHandle{}))
	astTestMust(t, state.AssignLiteral(a.Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, state.AssignLiteral(b.Negated(), NewSMTBooleanAxiom()))
	consistent, err := state.Propagate()
	astTestMust(t, err)
	if consistent || !state.Inconsistent() || state.conflict.Kind() != SMTBooleanJustificationBinaryClause {
		t.Fatal("binary conflict", consistent, state.Inconsistent(), state.conflict.Kind())
	}

	otherManager := astTestManager(t)
	otherContext, err := NewSMTCheckerContext(otherManager)
	astTestMust(t, err)
	other, err := NewSMTInternalizerState(otherManager, otherContext, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, other.Close()) })
	astTestMust(t, other.PushScope())
	if err = other.SetBinaryClauseOptimization(true); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("late binary optimization configuration", err)
	}
}

func TestSMTBinaryClauseOptimizationDefersScopedClauses(t *testing.T) {
	state, a, b := newSMTBinaryClauseFixture(t)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.addClause([]SMTLiteral{a, b}, false, ast.ASTHandle{}))
	if len(state.testClauses()) != 1 || state.testClauses()[0].Binary || state.testClauses()[0].clause == nil {
		t.Fatal("scoped clause used persistent binary representation", state.testClauses())
	}
	astTestMust(t, state.PopScopes(1))
	if len(state.testClauses()) != 0 {
		t.Fatal("scoped ordinary clause survived pop")
	}
}

func TestSMTKernelEnablesBinaryClausesUnlessProofsAreEnabled(t *testing.T) {
	m := astTestManager(t)
	kernel := smtTestKernel(t, m)
	state, err := kernel.Internalizer()
	astTestMust(t, err)
	if !state.binaryClauseOptimization {
		t.Fatal("ordinary kernel omitted source-default binary optimization")
	}

	proofManager := astTestManager(t)
	astTestMust(t, proofManager.ToggleProofMode(ast.ASTProofEnabled))
	proofKernel := smtTestKernel(t, proofManager)
	proofState, err := proofKernel.Internalizer()
	astTestMust(t, err)
	if proofState.binaryClauseOptimization {
		t.Fatal("proof kernel enabled pointer-free binary clauses")
	}
}
