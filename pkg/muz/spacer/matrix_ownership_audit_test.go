package spacer

import (
	"math/big"
	"testing"
)

func TestSpacerMatrixOwnershipAndSignedPivot(t *testing.T) {
	m := newSpacerMatrix(2, 2)
	input := big.NewRat(-2, 3)
	if err := m.set(1, 0, input); err != nil {
		t.Fatal(err)
	}
	input.SetInt64(7)
	got, err := m.get(1, 0)
	if err != nil || got.Cmp(big.NewRat(-2, 3)) != 0 {
		t.Fatalf("set aliases input: %v, %v", got, err)
	}
	got.SetInt64(9)
	again, err := m.get(1, 0)
	if err != nil || again.Cmp(big.NewRat(-2, 3)) != 0 {
		t.Fatalf("get aliases cell: %v, %v", again, err)
	}
	// The source selects the largest signed value: zero wins over -2/3,
	// so the first column is skipped rather than choosing a negative pivot.
	rank, err := m.gaussianElimination()
	if err != nil || rank != 0 {
		t.Fatalf("rank = %d, %v; want 0", rank, err)
	}
}
