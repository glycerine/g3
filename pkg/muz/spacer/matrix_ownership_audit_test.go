package spacer

import (
	"testing"
)

func TestSpacerMatrixOwnershipAndSignedPivot(t *testing.T) {
	manager, _ := newSpacerTestServices(t)
	q := manager.NumeralService().Manager
	m := newSpacerMatrix(q, 2, 2)
	defer m.del()
	input := spacerTestRational(t, q, -2, 3)
	if err := m.set(1, 0, input); err != nil {
		t.Fatal(err)
	}
	if err := q.SetInt64(input, 7); err != nil {
		t.Fatal(err)
	}
	got, err := m.get(1, 0)
	if err != nil || !spacerTestEqual(t, q, got, -2, 3) {
		t.Fatalf("set aliases input: %v, %v", got, err)
	}
	// The source selects the largest signed value: zero wins over -2/3,
	// so the first column is skipped rather than choosing a negative pivot.
	rank, err := m.gaussianElimination()
	if err != nil || rank != 0 {
		t.Fatalf("rank = %d, %v; want 0", rank, err)
	}
}
