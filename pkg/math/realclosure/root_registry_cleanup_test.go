package realclosure

import "testing"

func TestAlgebraicRegistryCleanupOnlyTrimsNilSuffix(t *testing.T) {
	f := newRankedValueFixture(t, true)
	first := newRegistryTestValue(t, f)
	last := newRegistryTestValue(t, f)
	owner := first.root.owner
	if err := f.base.store.release(last); err != nil {
		t.Fatal(err)
	}
	before := len(owner.roots)
	if err := owner.cleanupTrailingRoots(); err != nil {
		t.Fatal(err)
	}
	if len(owner.roots) != before-1 || owner.roots[first.root.index] != first.root || first.deleted {
		t.Fatal(before, len(owner.roots))
	}
}
