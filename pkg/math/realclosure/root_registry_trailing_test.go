package realclosure

import "testing"

func TestAlgebraicRegistryReusesTrailingDeletedIndex(t *testing.T) {
	f := newRankedValueFixture(t, true)
	first := newRegistryTestValue(t, f)
	second := newRegistryTestValue(t, f)
	deletedIndex := second.root.index
	if err := f.base.store.release(second); err != nil {
		t.Fatal(err)
	}
	replacement := newRegistryTestValue(t, f)
	if replacement.root.index != deletedIndex {
		t.Fatal(first.root.index, deletedIndex, replacement.root.index)
	}
}
