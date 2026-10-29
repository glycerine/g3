package smtcore

// Source: src/util/obj_pair_set.h and its sole retained production consumer,
// src/smt/smt_conflict_resolution.h.

import "testing"

func TestSMTConflictObjectPairSetCanonicalInsertionAndMembership(t *testing.T) {
	manager := astTestManager(t)
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	_, first := smtEnodeConstant(t, manager, 84601, boolean, 0)
	_, second := smtEnodeConstant(t, manager, 84602, boolean, 0)
	expander := NewSMTAntecedentExpander()
	expander.active = true

	if err := expander.MarkEquality(second, first); err != nil {
		t.Fatal(err)
	}
	if err := expander.MarkEquality(first, second); err != nil {
		t.Fatal(err)
	}
	if len(expander.equalities) != 1 || len(expander.processed) != 1 {
		t.Fatalf("insert_if_not_there did not suppress the reversed duplicate: todo=%d set=%d", len(expander.equalities), len(expander.processed))
	}
	key, err := expander.canonicalPair(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := expander.processed[key]; !found {
		t.Fatal("contains did not find the canonical owner-ID pair")
	}
	if got := expander.equalities[0]; got.First != key.first || got.Second != key.second {
		t.Fatal("todo pair was not published in canonical owner-ID order")
	}
}

func TestSMTConflictObjectPairSetResetAllowsReuse(t *testing.T) {
	manager := astTestManager(t)
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	_, first := smtEnodeConstant(t, manager, 84611, boolean, 0)
	_, second := smtEnodeConstant(t, manager, 84612, boolean, 0)
	expander := NewSMTAntecedentExpander()
	expander.active = true
	if err := expander.MarkEquality(first, second); err != nil {
		t.Fatal(err)
	}
	if err := expander.cleanup(); err != nil {
		t.Fatal(err)
	}
	if len(expander.processed) != 0 || len(expander.equalities) != 0 || expander.active {
		t.Fatal("reset retained pair membership or todo state")
	}

	expander.active = true
	if err := expander.MarkEquality(second, first); err != nil {
		t.Fatal(err)
	}
	if len(expander.processed) != 1 || len(expander.equalities) != 1 {
		t.Fatal("pair set could not be reused after reset")
	}
}
