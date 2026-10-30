package realclosure

import "testing"

func TestAlgebraicRegistryLiveEnumerationSkipsInteriorHoles(t *testing.T) {
	f := newRankedValueFixture(t, true)
	first := newRegistryTestValue(t, f)
	middle := newRegistryTestValue(t, f)
	last := newRegistryTestValue(t, f)
	if err := f.base.store.release(middle); err != nil {
		t.Fatal(err)
	}
	live, err := first.root.owner.liveAlgebraicRoots()
	if err != nil {
		t.Fatal(err)
	}
	foundFirst, foundLast := false, false
	for _, root := range live {
		foundFirst = foundFirst || root == first.root
		foundLast = foundLast || root == last.root
		if root == middle.root {
			t.Fatal("deleted root enumerated")
		}
	}
	if !foundFirst || !foundLast {
		t.Fatal(foundFirst, foundLast)
	}
}
