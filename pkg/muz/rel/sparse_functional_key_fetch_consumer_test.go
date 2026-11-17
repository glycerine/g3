package rel

import "testing"

func TestSparseFunctionalZeroKeyFetch(t *testing.T) {
	signature, err := NewSignature([]uint64{16})
	astTestMust(t, err)
	index, err := NewSparseTable(signature, 1)
	astTestMust(t, err)
	defer func() { astTestMust(t, index.Close()) }()
	if _, err = index.Add(Fact{7}); err != nil {
		t.Fatal(err)
	}
	suffix, found, err := index.FetchFunctionalKey(nil)
	if err != nil || !found || len(suffix) != 1 || suffix[0] != 7 {
		t.Fatal("zero-key functional fetch", suffix, found, err)
	}
	suffix[0] = 3
	again, found, err := index.FetchFunctionalKey(nil)
	if err != nil || !found || again[0] != 7 {
		t.Fatal("functional suffix was borrowed", again, found, err)
	}
	if _, _, err = index.FetchFunctionalKey(Fact{0}); err == nil {
		t.Fatal("zero-key index accepted a spurious key column")
	}

}
