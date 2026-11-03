package rel

import "testing"

func TestSparseFunctionalJoinProjectFallbackPreservesEquivalentKeySuffix(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 8}, 1,
		Fact{1, 2}, Fact{2, 3})
	right := newSparseFunctionalFixture(t, []uint64{4, 8}, 1,
		Fact{1, 2}, Fact{2, 3})
	// Joining column 1 makes the indexed sparse specialization decline. The
	// joined column-0 pair is also equal, so removing the left copy cannot
	// merge rows and source preserves both functional suffixes.
	finite, err := finiteTableJoinProject(left, right, []int{0, 1}, []int{0, 1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := finite.(*SparseTable)
	if !ok {
		t.Fatalf("compiled join-project fallback type %T, want Sparse", finite)
	}
	defer result.Close()
	if functional, cause := result.FunctionalColumns(); cause != nil || functional != 2 {
		t.Fatalf("functional fallback columns=%d err=%v, want 2", functional, cause)
	}
	for _, fact := range []Fact{{1, 2, 2}, {2, 3, 3}} {
		if exact, cause := result.ContainsExact(fact); cause != nil || !exact {
			t.Fatal("functional fallback result missing", fact, exact, cause)
		}
	}
}
