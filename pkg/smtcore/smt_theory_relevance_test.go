package smtcore

import "testing"

func TestSMTTheoryAssignmentsWaitForRelevanceAndRestoreOnPop(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	theory := &smtInternalizerTestTheory{SMTTheoryBase: base}
	_, relevance, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	state.relevancyLevel = 2
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	sort, err := m.GetSort(zero)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "relevance-assignment-x"), sort)
	astTestMust(t, err)
	atom, err := m.MkArithmeticLE(x, zero)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(atom, true))
	literal, err := state.Literal(atom)
	astTestMust(t, err)
	for repeat := 0; repeat < 2; repeat++ {
		astTestMust(t, state.PushScope())
		assigned := literal
		if repeat == 1 {
			assigned = assigned.Negated()
		}
		astTestMust(t, state.AssignLiteral(assigned, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		if len(theory.assignments) != repeat {
			t.Fatal("irrelevant atom reached the theory")
		}
		astTestMust(t, relevance.MarkRelevant(atom))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		_, err = state.PropagateCore()
		astTestMust(t, err)
		if len(theory.assignments) != repeat+1 {
			t.Fatal("theory assignment was lost or duplicated")
		}
		if got := theory.assignments[repeat]; got.variable != literal.Var() || got.value != (repeat == 0) {
			t.Fatal("theory assignment polarity", got)
		}
		astTestMust(t, state.PopScopes(1))
		if relevance.IsRelevant(atom) || state.atomIsNotified(literal.Var()) {
			t.Fatal("scoped relevance notification survived pop")
		}
	}
	// Source relevance levels below two notify assigned non-quantifier atoms
	// without waiting for a relevance event.
	state.relevancyLevel = 1
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if len(theory.assignments) != 3 {
		t.Fatal("level-one atom was deferred")
	}
}
