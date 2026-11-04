package rel

import (
	"math/big"
	"testing"
)

func TestKarrSourceInterpretedRetainsCachedBasis(t *testing.T) {
	r := NewKarrEmpty(1, nil)
	if err := r.AddFact([]*big.Rat{big.NewRat(1, 1)}); err != nil {
		t.Fatal(err)
	}
	basis, err := r.Basis()
	if err != nil {
		t.Fatal(err)
	}
	if err = KarrSourceFilterConstraint(r, []big.Rat{*big.NewRat(-1, 1)}, big.NewRat(2, 1), false); err != nil {
		t.Fatal(err)
	}
	after, err := r.Basis()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := r.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	if basis.Len() != 1 || after.Len() != 1 || after.A[0][0].Cmp(&basis.A[0][0]) != 0 || rows.Len() != 2 || rows.B[1].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("interpreted cache", basis, after, rows)
	}
	empty := NewKarrEmpty(1, nil)
	if err = KarrSourceFilterConstraint(empty, []big.Rat{*big.NewRat(1, 1)}, nil, true); err != nil || !empty.Empty() {
		t.Fatal("empty filter", err)
	}
}
