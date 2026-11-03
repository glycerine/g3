package rel

import "testing"

func TestMixedHashSparseJoinProjectPreservesEquivalentKeySuffix(t *testing.T) {
	hashSignature, err := NewSignature([]uint64{4})
	if err != nil {
		t.Fatal(err)
	}
	for _, sparseFirst := range []bool{false, true} {
		name := "hash-sparse"
		if sparseFirst {
			name = "sparse-hash"
		}
		t.Run(name, func(t *testing.T) {
			hash := NewHashTable(hashSignature)
			if _, cause := hash.Add(Fact{1}); cause != nil {
				t.Fatal(cause)
			}
			defer hash.Close()
			sparse := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 7})
			var first, second FiniteTable = hash, sparse
			if sparseFirst {
				first, second = sparse, hash
			}
			finite, cause := finiteTableJoinProject(first, second, []int{0}, []int{0}, []int{0})
			if cause != nil {
				t.Fatal(cause)
			}
			result, ok := finite.(*SparseTable)
			if !ok {
				t.Fatalf("mixed fallback type %T, want Sparse", finite)
			}
			defer result.Close()
			if functional, cause := result.FunctionalColumns(); cause != nil || functional != 1 {
				t.Fatalf("mixed fallback functional=%d err=%v, want 1", functional, cause)
			}
			if exact, cause := result.ContainsExact(Fact{1, 7}); cause != nil || !exact {
				t.Fatal("mixed fallback result missing", exact, cause)
			}
		})
	}
}
