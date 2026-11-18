package rel

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/math/hilbert"
)

func TestKarrDualizationOverflowPreservesOwners(t *testing.T) {
	largeInt := new(big.Int).Lsh(big.NewInt(1), 63)
	large := new(big.Rat).SetInt(largeInt)
	source := NewKarrEmpty(karrTestQ(), 1, nil)
	if err := source.AddFactRat([]*big.Rat{large}); err != nil {
		t.Fatal(err)
	}
	target := NewKarrEmpty(karrTestQ(), 1, nil)
	if err := target.AddFactRat([]*big.Rat{big.NewRat(4, 1)}); err != nil {
		t.Fatal(err)
	}
	before, err := target.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := source.Basis(); !errors.Is(err, hilbert.ErrOverflow) {
			t.Fatal("large fact basis", err)
		}
		if changed, err := target.Union(source, nil); changed || !errors.Is(err, hilbert.ErrOverflow) {
			t.Fatal("union", changed, err)
		}
		after, err := target.Inequalities()
		if err != nil {
			t.Fatal(err)
		}
		if after.Len() != before.Len() || after.B[0].Cmp(&before.B[0]) != 0 || source.basisValid {
			t.Fatal("failed basis conversion changed relation")
		}
	}
	small := NewKarrEmpty(karrTestQ(), 1, nil)
	if err := small.AddFactRat([]*big.Rat{big.NewRat(5, 1)}); err != nil {
		t.Fatal(err)
	}
	if changed, err := target.Union(small, nil); err != nil || !changed {
		t.Fatal("valid union after refusal", changed, err)
	}

	// A basis-to-inequality conversion has the same checked arithmetic boundary.
	homogeneous := NewKarrFull(karrTestQ(), 1, nil)
	homogeneous.inequalitiesValid = false
	homogeneous.basisValid = true
	if err := karrPushBig(homogeneous.q, &homogeneous.basis, []big.Rat{*large}, big.NewRat(0, 1), true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := homogeneous.Inequalities(); !errors.Is(err, hilbert.ErrOverflow) {
			t.Fatal("large homogeneous row", err)
		}
		entry, err := karrBig(homogeneous.q, &homogeneous.basis.A[0][0])
		if err != nil || homogeneous.inequalitiesValid || entry.Cmp(large) != 0 {
			t.Fatal("failed homogeneous conversion changed basis")
		}
	}
	if err := homogeneous.q.SetInt64(&homogeneous.basis.A[0][0], 1); err != nil {
		t.Fatal(err)
	}
	if _, err := homogeneous.Inequalities(); err != nil {
		t.Fatal("homogeneous retry", err)
	}
}
