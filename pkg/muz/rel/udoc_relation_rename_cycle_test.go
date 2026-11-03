package rel

import "testing"

func TestUDocRenameThreeCyclePreservesMixedWidthsAndComplement(t *testing.T) {
	manager, _ := newTestDocManager(t, 6)
	source, err := NewUDocRelation(manager, []uint32{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, source)
	if _, err = source.AddFact([]uint64{1, 2, 5}); err != nil {
		t.Fatal(err)
	}
	complement, err := source.Complement()
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, complement)
	for _, original := range []*UDocRelation{source, complement} {
		renamed, err := original.Rename([]uint32{0, 1, 2})
		if err != nil {
			t.Fatal(err)
		}
		closeUDoc(t, renamed)
		for column, want := range []uint32{2, 3, 1} {
			if width, err := renamed.ColumnNumBits(uint32(column)); err != nil || width != want {
				t.Fatal("three-cycle column width", column, width, want, err)
			}
		}
		// Source contains_fact is conservative for negative cubes; the
		// independent bit oracle checks every positive and negative assignment.
		assertUnionTruth(t, renamed.elems, func(assignment uint32) bool {
			return (assignment == 2|(5<<2)|(1<<5)) == (original == source)
		})
		assertUnionTruth(t, original.elems, func(assignment uint32) bool {
			return (assignment == 1|(2<<1)|(5<<3)) == (original == source)
		})
		roundtrip, err := renamed.Rename([]uint32{2, 1, 0})
		if err != nil {
			t.Fatal(err)
		}
		closeUDoc(t, roundtrip)
		assertUnionTruth(t, roundtrip.elems, func(assignment uint32) bool {
			return (assignment == 1|(2<<1)|(5<<3)) == (original == source)
		})
	}
}
