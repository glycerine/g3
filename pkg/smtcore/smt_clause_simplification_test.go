package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Source: src/smt/smt_internalizer.cpp context::mk_clause case 0 retains j
// as the conflict justification, including after auxiliary simplification.
func TestSMTEmptyClauseRetainsResolvedProof(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	_, _, state := smtInternalizerFixture(t, m, nil)
	p := smtRelevancyBool(t, m, "empty-proof-p")
	notP, err := m.MkNot(p)
	astTestMust(t, err)
	positive, err := m.MkProofAsserted(p)
	astTestMust(t, err)
	negative, err := m.MkProofAsserted(notP)
	astTestMust(t, err)
	proof, err := m.MkProofUnitResolution([]ast.ASTHandle{positive, negative})
	astTestMust(t, err)
	astTestMust(t, state.addClause(nil, false, proof))
	conflict, found := state.Conflict()
	if !found || conflict.Kind() != SMTBooleanJustificationObject {
		t.Fatal("empty clause lost its proof justification")
	}
	resolver, err := NewSMTConflictResolver(state)
	astTestMust(t, err)
	defer func() { astTestMust(t, resolver.Close()) }()
	resolution, err := resolver.Resolve(conflict, state.ConflictLiteral(), state.scopeLevel)
	astTestMust(t, err)
	if resolution.Resolved {
		t.Fatal("base conflict unexpectedly resolved")
	}
	retained, err := resolver.LemmaProof()
	astTestMust(t, err)
	if retained != proof {
		t.Fatal("refutation replaced by an open hypothesis")
	}
}

func TestSMTClauseSimplificationDropsFalseTailLiteral(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	expressions := []ast.ASTHandle{
		smtRelevancyBool(t, m, "simplify-tail-a"),
		smtRelevancyBool(t, m, "simplify-tail-b"),
		smtRelevancyBool(t, m, "simplify-tail-c"),
	}
	literals := make([]SMTLiteral, len(expressions))
	for index, expression := range expressions {
		astTestMust(t, state.Internalize(expression, true))
		variable, _ := state.BoolVar(expression)
		literals[index] = NewSMTLiteral(variable, false)
	}
	astTestMust(t, state.addClauseKind(literals, false, SMTClauseLearned, ast.ASTHandle{}))
	astTestMust(t, state.AssignLiteral(literals[2].Negated(), NewSMTBooleanAxiom()))
	consistent, err := state.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("unexpected conflict before simplification")
	}
	astTestMust(t, state.simplifyClausesAtBase(0))
	if len(state.testClauses()) != 1 || len(state.testClauses()[0].clause.literals) != 2 {
		t.Fatal("false tail literal was not removed")
	}
	for _, literal := range state.testClauses()[0].clause.literals {
		if literal == literals[2] {
			t.Fatal("false tail literal survived simplification")
		}
	}
}

func TestSMTClauseSimplificationDeletesSatisfiedReasonSafely(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	first := smtRelevancyBool(t, m, "simplify-reason-a")
	second := smtRelevancyBool(t, m, "simplify-reason-b")
	for _, expression := range []ast.ASTHandle{first, second} {
		astTestMust(t, state.Internalize(expression, true))
	}
	firstVar, _ := state.BoolVar(first)
	secondVar, _ := state.BoolVar(second)
	firstLiteral := NewSMTLiteral(firstVar, false)
	secondLiteral := NewSMTLiteral(secondVar, false)
	astTestMust(t, state.addClause([]SMTLiteral{firstLiteral, secondLiteral}, false, ast.ASTHandle{}))
	astTestMust(t, state.AssignLiteral(secondLiteral.Negated(), NewSMTBooleanAxiom()))
	consistent, err := state.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("unexpected propagation conflict")
	}
	if clause, ok := state.booleanData[firstVar].Justification().Clause(); !ok || clause == nil {
		t.Fatal("clause did not justify propagated literal")
	}
	astTestMust(t, state.simplifyClausesAtBase(0))
	if len(state.testClauses()) != 0 {
		t.Fatal("satisfied clause was not deleted")
	}
	if state.booleanData[firstVar].Justification().Kind() != SMTBooleanJustificationAxiom {
		t.Fatal("deleted clause remained an assignment reason")
	}
}

func TestSMTClauseSimplificationRetainsProofReasons(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	_, _, state := smtInternalizerFixture(t, m, nil)
	first := smtRelevancyBool(t, m, "simplify-proof-a")
	second := smtRelevancyBool(t, m, "simplify-proof-b")
	for _, expression := range []ast.ASTHandle{first, second} {
		astTestMust(t, state.Internalize(expression, true))
	}
	firstVar, _ := state.BoolVar(first)
	secondVar, _ := state.BoolVar(second)
	firstLiteral := NewSMTLiteral(firstVar, false)
	secondLiteral := NewSMTLiteral(secondVar, false)
	fact, err := m.MkOr(first, second)
	astTestMust(t, err)
	proof, err := m.MkProofHypothesis(fact)
	astTestMust(t, err)
	astTestMust(t, state.addClause([]SMTLiteral{firstLiteral, secondLiteral}, false, proof))
	astTestMust(t, state.AssignLiteral(secondLiteral.Negated(), NewSMTBooleanAxiom()))
	consistent, err := state.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("unexpected propagation conflict")
	}
	astTestMust(t, state.simplifyClausesAtBase(0))
	justification, ok := state.booleanData[firstVar].Justification().Justification()
	if !ok {
		t.Fatal("proof reason was replaced by an axiom")
	}
	unit, ok := justification.(*SMTUnitResolutionJustification)
	if !ok || len(unit.Literals) != 1 || unit.Literals[0] != secondLiteral.Negated() {
		t.Fatal("unexpected simplified proof reason", justification)
	}
	if wrapper, ok := unit.Antecedent.(*SMTJustificationProofWrapper); !ok {
		t.Fatal("clause proof was not retained")
	} else if _, err = wrapper.Proof(); err != nil {
		t.Fatal("retained clause proof is closed", err)
	}
}

func TestSMTClauseSimplificationRetainsWrappedRegionProofReason(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	_, _, state := smtInternalizerFixture(t, m, nil)
	var expressions []ast.ASTHandle
	var literals []SMTLiteral
	for _, name := range []string{"wrapped-reason-a", "wrapped-reason-b", "wrapped-reason-c"} {
		expression := smtRelevancyBool(t, m, name)
		astTestMust(t, state.Internalize(expression, true))
		variable, _ := state.BoolVar(expression)
		expressions = append(expressions, expression)
		literals = append(literals, NewSMTLiteral(variable, false))
	}
	fact, err := m.MkOrArgs(expressions)
	astTestMust(t, err)
	proof, err := m.MkProofHypothesis(fact)
	astTestMust(t, err)
	astTestMust(t, state.addClause(literals, false, proof))
	astTestMust(t, state.AssignLiteral(literals[2].Negated(), NewSMTBooleanAxiom()))
	consistent, err := state.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("unexpected first propagation conflict")
	}
	deleted, err := state.simplifyClause(&state.auxClauses[0])
	astTestMust(t, err)
	if deleted {
		t.Fatal("partially simplified clause was deleted")
	}
	if _, wrapped := state.testClauses()[0].clause.Justification().(*SMTUnitResolutionJustification); !wrapped {
		t.Fatal("false tail did not wrap the original region proof")
	}
	astTestMust(t, state.AssignLiteral(literals[1].Negated(), NewSMTBooleanAxiom()))
	consistent, err = state.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("unexpected second propagation conflict")
	}
	astTestMust(t, state.simplifyClausesAtBase(0))
	if len(state.testClauses()) != 0 {
		t.Fatal("satisfied clause was not deleted")
	}
	justification, found := state.booleanData[literals[0].Var()].Justification().Justification()
	if !found {
		t.Fatal("missing surviving propagation reason")
	}
	for {
		if unit, ok := justification.(*SMTUnitResolutionJustification); ok {
			justification = unit.Antecedent
			continue
		}
		wrapper, ok := justification.(*SMTJustificationProofWrapper)
		if !ok {
			t.Fatal("missing original proof wrapper")
		}
		retained, err := wrapper.Proof()
		astTestMust(t, err)
		if retained != proof {
			t.Fatal("original proof was replaced")
		}
		break
	}
	builder := &smtConflictProofBuilder{state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle), objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle)}
	result, err := builder.literalProof(literals[0])
	astTestMust(t, err)
	got, err := m.GetProofFact(result)
	astTestMust(t, err)
	if got != expressions[0] {
		t.Fatal("surviving proof has the wrong fact")
	}
}
