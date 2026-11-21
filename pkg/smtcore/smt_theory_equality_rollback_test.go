package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

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

// A failed second deletion must leave the owning trail entry available.
func TestSMTTheoryAttachmentPopRetriesRoot(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, values := smtUFConstants(t, m, "retry-theory-root", 2)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTDummyTheory(family, "rollback")
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	a, err := state.MkEnode(values[0], false, false, true)
	astTestMust(t, err)
	b, err := state.MkEnode(values[1], false, false, true)
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.addEquality(a, b, NewSMTEqualityAxiom()))
	root := a.Root()
	child := a
	if root == a {
		child = b
	}
	astTestMust(t, state.attachTheoryVariable(child, family, 11))
	astTestMust(t, root.DeleteTheoryVar(family))
	if err := state.PopScopes(1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("missing root attachment returned %v", err)
	}
	if child.TheoryVar(family) != SMTNullTheoryVar || child.Root() != root || state.pendingPop == nil || len(state.theoryTrail) != 1 {
		t.Fatal("destructive prefix lost the pending attachment or its equality class")
	}
	astTestMust(t, root.AddTheoryVar(family, 11))
	astTestMust(t, state.PopScopes(1))
	if a.Root() == b.Root() || a.TheoryVar(family) != SMTNullTheoryVar || b.TheoryVar(family) != SMTNullTheoryVar || state.pendingPop != nil || state.scopeLevel != 0 {
		t.Fatal("pop retry failed to restore the outer scope")
	}
}
