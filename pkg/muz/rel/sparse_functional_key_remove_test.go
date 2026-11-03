package rel

import "testing"

func TestSparseFunctionalRemoveUsesZeroColumnKey(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{8}, 1, Fact{3})
	removed, err := table.RemoveFacts([]Fact{{7}})
	if err != nil || removed != 1 {
		t.Fatal("zero-key bulk removal", removed, err)
	}
	if rows, cause := table.Len(); cause != nil || rows != 0 {
		t.Fatal("zero-key row survived removal", rows, cause)
	}
	missing, err := table.RemoveFunctionalKey(nil)
	if err != nil || missing {
		t.Fatal("missing zero-key removal", missing, err)
	}
	if _, err = table.RemoveFunctionalKey(Fact{0}); err == nil {
		t.Fatal("zero-key table accepted a spurious key column")
	}
	if _, err = table.RemoveFacts([]Fact{{8}}); err == nil {
		t.Fatal("out-of-domain suffix bypassed public row validation")
	}
}
