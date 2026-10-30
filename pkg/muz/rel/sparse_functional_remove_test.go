package rel

import "testing"

func TestSparseFunctionalBulkRemoveUsesKeySemanticsAndStopsOnFailure(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 2}, Fact{2, 3})
	removed, err := table.RemoveFacts([]Fact{{1, 7}, {3, 0}, {2, 6}})
	if err != nil || removed != 2 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	if length, _ := table.Len(); length != 0 {
		t.Fatalf("remaining=%d", length)
	}
	table = newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 2})
	removed, err = table.RemoveFacts([]Fact{{1, 2}, {9, 0}})
	if err == nil || removed != 1 {
		t.Fatalf("partial removed=%d err=%v", removed, err)
	}
}
