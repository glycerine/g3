package smtcore

import "testing"

// AUDIT.md smtcore_08-F3: context::propagate checks get_cancel_flag() after
// the case splits and resource_limits_exceeded() after the quantifier
// manager, each charging the resource limit, and rolls m_qhead back when it
// stops early.
func TestSMTPropagateCoreChecksCancelLikeContextPropagate(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	theory := &smtInternalizerTestTheory{SMTTheoryBase: base}
	_, _, s := smtInternalizerFixture(t, m, nil)
	astTestMust(t, s.RegisterTheory(theory))
	a := smtRelevancyBool(t, m, "prop-cancel-a")
	astTestMust(t, s.Internalize(a, true))
	lit, err := s.Literal(a)
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(lit, NewSMTBooleanAxiom()))

	head := s.propagationHead
	m.Limit.Cancel()
	ok, err := s.PropagateCore()
	astTestMust(t, err)
	if !ok || s.propagationHead != head || theory.propagationCalled || s.relevancyBatch.active {
		t.Fatal("canceled propagation", ok, s.propagationHead, head, theory.propagationCalled)
	}
	m.Limit.ResetCancel()
	before := m.Limit.Count()
	ok, err = s.PropagateCore()
	astTestMust(t, err)
	if !ok || !theory.propagationCalled || s.propagationHead == head {
		t.Fatal("resumed propagation", ok, theory.propagationCalled, s.propagationHead)
	}
	// One round: get_cancel_flag() and resource_limits_exceeded()'s
	// get_cancel_flag() each charge one unit.
	if got := m.Limit.Count() - before; got != 2 {
		t.Fatal("propagation round resource charge", got, "want 2")
	}
}
