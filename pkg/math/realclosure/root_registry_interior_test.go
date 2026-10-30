package realclosure

import "testing"

func TestAlgebraicRegistryPreservesInteriorHole(t *testing.T) {
	f := newRankedValueFixture(t, true)
	first := newRegistryTestValue(t, f)
	middle := newRegistryTestValue(t, f)
	last := newRegistryTestValue(t, f)
	middleIndex, priorLength := middle.root.index, len(first.root.owner.roots)
	if err := f.base.store.release(middle); err != nil {
		t.Fatal(err)
	}
	replacement := newRegistryTestValue(t, f)
	if replacement.root.index != priorLength || replacement.root.index == middleIndex || last.deleted {
		t.Fatal(middleIndex, priorLength, replacement.root.index)
	}
}
