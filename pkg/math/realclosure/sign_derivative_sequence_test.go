package realclosure

import "testing"

func TestRationalDerivativeSequenceSourceOrder(t *testing.T) {
	f := newBoundedSignFixture(t, -3, 3)
	one, minusFive, four := f.rational(1), f.rational(-5), f.rational(4)
	p := []*realValue{four, nil, minusFive, nil, one}
	rows, err := f.store.derivativeSequenceRational(p)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]int64{{0, -10, 0, 4}, {-10, 0, 12}, {0, 24}, {24}}
	if len(rows) != len(want) {
		t.Fatalf("derivative rows=%d, want %d", len(rows), len(want))
	}
	for i, expected := range want {
		if len(rows[i].coefficients) != len(expected) {
			t.Fatalf("row %d size=%d, want %d", i, len(rows[i].coefficients), len(expected))
		}
		for j, n := range expected {
			value := rows[i].coefficients[j]
			if n == 0 {
				if value != nil {
					t.Fatalf("row %d coefficient %d is nonzero", i, j)
				}
				continue
			}
			if value == nil {
				t.Fatalf("row %d coefficient %d is zero", i, j)
			}
			got, err := f.store.numbers.Int64(&value.rational)
			if err != nil || got != n {
				t.Fatalf("row %d coefficient %d=%d, want %d, err=%v", i, j, got, n, err)
			}
		}
	}
	if err := f.store.closePolynomialSequence(rows); err != nil {
		t.Fatal(err)
	}
}
