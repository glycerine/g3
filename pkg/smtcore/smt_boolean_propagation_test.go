package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// smtcore_01-F6: context::bcp calls get_cancel_flag() (one resource step)
// before each dequeued literal and stops, leaving m_qhead, when canceled.
func TestSMTPropagateCountsResourcePerLiteral(t *testing.T) {
	m := astTestManager(t)
	expressions := []ast.ASTHandle{
		smtRelevancyBool(t, m, "bcp-rl-a"), smtRelevancyBool(t, m, "bcp-rl-b"), smtRelevancyBool(t, m, "bcp-rl-c"),
		smtRelevancyBool(t, m, "bcp-rl-d"), smtRelevancyBool(t, m, "bcp-rl-e"),
	}
	_, _, state := smtInternalizerFixture(t, m, nil)
	literals := make([]SMTLiteral, len(expressions))
	for i, expression := range expressions {
		astTestMust(t, state.Internalize(expression, true))
		var err error
		literals[i], err = state.Literal(expression)
		astTestMust(t, err)
	}
	astTestMust(t, state.PushScope())
	for i := 0; i+1 < len(literals); i++ {
		astTestMust(t, state.addClause([]SMTLiteral{literals[i].Negated(), literals[i+1]}, false, ast.ASTHandle{}))
	}
	astTestMust(t, state.AssignLiteral(literals[0], NewSMTBooleanAxiom()))
	pending := len(state.assignedLiterals) - state.propagationHead
	before := m.Limit.Count()
	consistent, err := state.Propagate()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("unexpected conflict")
	}
	dequeued := pending + len(literals) - 1
	if got := m.Limit.Count() - before; got != uint64(dequeued) {
		t.Fatalf("resource steps = %d, want %d", got, dequeued)
	}
	astTestMust(t, state.PopScopes(1))

	astTestMust(t, state.PushScope())
	for i := 0; i+1 < len(literals); i++ {
		astTestMust(t, state.addClause([]SMTLiteral{literals[i].Negated(), literals[i+1]}, false, ast.ASTHandle{}))
	}
	astTestMust(t, state.AssignLiteral(literals[0], NewSMTBooleanAxiom()))
	head := state.propagationHead
	m.Limit.Cancel()
	t.Cleanup(m.Limit.ResetCancel)
	consistent, err = state.Propagate()
	m.Limit.ResetCancel()
	astTestMust(t, err)
	if !consistent || state.propagationHead != head || state.propagationHead >= len(state.assignedLiterals) {
		t.Fatal("canceled bcp must return true without dequeuing", consistent, head, state.propagationHead)
	}
	astTestMust(t, state.PopScopes(1))
}
