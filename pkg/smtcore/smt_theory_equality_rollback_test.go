package smtcore

import "testing"

func TestSMTTheoryAttachmentsInterleavedWithEqualityRollback(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, values := smtUFConstants(t, m, "theory-equality-rollback", 3)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTDummyTheory(family, "rollback")
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	nodes := make([]*SMTEnode, len(values))
	for i, value := range values {
		nodes[i], err = state.MkEnode(value, false, false, true)
		astTestMust(t, err)
	}
	astTestMust(t, state.attachTheoryVariable(nodes[2], family, 7))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.addEquality(nodes[0], nodes[1], NewSMTEqualityAxiom()))
	astTestMust(t, state.attachTheoryVariable(nodes[0], family, 11))
	if nodes[0].TheoryVar(family) != 11 || nodes[1].TheoryVar(family) != 11 {
		t.Fatal("post-merge theory variable was not attached to the class")
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, state.addEquality(nodes[1], nodes[2], NewSMTEqualityAxiom()))
	astTestMust(t, state.attachTheoryVariable(nodes[0], family, 19))
	astTestMust(t, state.PopScopes(1))
	if nodes[0].root != nodes[1].root || nodes[2].root == nodes[0].root || nodes[0].TheoryVar(family) != 11 || nodes[1].TheoryVar(family) != 11 || nodes[2].TheoryVar(family) != 7 {
		t.Fatal("inner scope did not restore theory variables and equality classes")
	}
	astTestMust(t, state.PopScopes(1))
	if nodes[0].root == nodes[1].root || nodes[0].TheoryVar(family) != SMTNullTheoryVar || nodes[1].TheoryVar(family) != SMTNullTheoryVar || nodes[2].TheoryVar(family) != 7 {
		t.Fatal("outer scope did not restore theory variables and equality classes")
	}
}
