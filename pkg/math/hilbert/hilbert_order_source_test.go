package hilbert

import "testing"

func TestSourceBasisIsSortedByCoordinateMagnitude(t *testing.T) {
	b := New(testQ(), nil)
	if err := b.AddEQ(rationals(1, 1, -2), rational(0)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Saturate(); err != nil {
		t.Fatal(err)
	}
	var previous uint64
	for index, value := range b.basis {
		magnitude, err := vectorMagnitude(value)
		if err != nil {
			t.Fatal(err)
		}
		if index != 0 && magnitude < previous {
			t.Fatalf("basis magnitude %d follows %d", magnitude, previous)
		}
		previous = magnitude
	}
}

// TestBasisSortMatchesStdSortForLargeTies: saturate's final
// std::sort(m_basis, vector_lt_t) (hilbert_basis.cpp:981) on 20 vectors
// with tied magnitudes must leave the permutation g++ 13.3's std::sort
// produces (recorded from a small C++ program with the same comparator;
// AUDIT.md math_misc_01-F4).
func TestBasisSortMatchesStdSortForLargeTies(t *testing.T) {
	magnitudes := []int64{3, 2, 3, 1, 2, 3, 3, 2, 1, 3, 2, 3, 3, 1, 2, 3, 2, 3, 1, 3}
	want := []int{18, 3, 13, 8, 1, 16, 14, 10, 7, 4, 9, 6, 11, 12, 5, 15, 2, 17, 0, 19}
	original := make([]*vector, len(magnitudes))
	for i, m := range magnitudes {
		// Alternate signs: vector_lt sums absolute values.
		if i%2 == 1 {
			m = -m
		}
		original[i] = &vector{values: []int64{m, 0}}
	}
	values := append([]*vector(nil), original...)
	if err := orderBasis(values); err != nil {
		t.Fatal(err)
	}
	for i, j := range want {
		if values[i] != original[j] {
			t.Fatalf("position %d: want vector %d", i, j)
		}
	}
}
