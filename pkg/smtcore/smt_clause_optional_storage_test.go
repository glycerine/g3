package smtcore

import (
	"runtime"
	"testing"
)

func TestSMTClauseOptionalProofAndHandlerSurviveCopies(t *testing.T) {
	m := astTestManager(t)
	first, second := new(smtClauseTestJustification), new(smtClauseTestJustification)
	handler := new(smtClauseTestDeleteHandler)
	literals := []SMTLiteral{NewSMTLiteral(1, false), NewSMTLiteral(2, true), NewSMTLiteral(3, false)}
	clause, err := NewSMTClause(m, literals, SMTClauseAux, first, handler, false, nil)
	astTestMust(t, err)
	original := *clause
	astTestMust(t, clause.SetJustification(second))
	updated := *clause
	astTestMust(t, clause.MarkDeleted())
	if handler.calls != 1 {
		t.Fatal("deletion callback count", handler.calls)
	}
	astTestMust(t, clause.SetJustification(nil))
	astTestMust(t, clause.Close())
	runtime.GC()
	if original.Justification() != first || updated.Justification() != second || clause.Justification() != nil || handler.calls != 1 {
		t.Fatal("proof identity or callback count changed across copies")
	}
	// An interface holding a typed nil is a supplied proof.
	var typedNil *smtClauseTestJustification
	other, err := NewSMTClause(m, literals, SMTClauseAux, typedNil, nil, false, nil)
	astTestMust(t, err)
	if other.Justification() == nil || other.Justification() != typedNil {
		t.Fatal("lost typed nil proof")
	}
	astTestMust(t, other.SetJustification(second))
	astTestMust(t, other.Close())
}
