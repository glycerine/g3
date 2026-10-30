package opt

// Source: src/opt/sortmax.cpp m_trail reverse ownership.

import "testing"

func TestMaxSMTASTOwnerHoldsAndReleasesExpressions(t *testing.T) {
	fixture := newWeightedSoftFixture(t)
	owner := maxSMTASTOwner{manager: fixture.manager}
	if _, err := owner.hold(fixture.p, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.hold(fixture.q, nil); err != nil {
		t.Fatal(err)
	}
	if len(owner.refs) != 2 {
		t.Fatal("owner refs", len(owner.refs))
	}
	if err := owner.close(); err != nil || owner.refs != nil {
		t.Fatal("owner close", err, owner.refs)
	}
}
