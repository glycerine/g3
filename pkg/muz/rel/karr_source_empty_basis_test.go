package rel

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/math/hilbert"
)

func TestKarrSourceFullProjectionAndUnion(t *testing.T) {
	calls := 0
	full := NewKarrFull(2, func() bool { calls++; return true })
	projected, err := KarrSourceProject(full, []uint32{0})
	if err != nil {
		t.Fatal(err)
	}
	basis, err := projected.Basis()
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || basis.Len() != 0 {
		t.Fatal("native full projection", calls, basis)
	}
	seed := NewKarrEmpty(1, nil)
	if err := seed.AddFact([]*big.Rat{big.NewRat(7, 1)}); err != nil {
		t.Fatal(err)
	}
	changed, err := KarrSourceUnion(seed, projected)
	if err != nil || changed {
		t.Fatal("union", changed, err)
	}
	matrix, err := seed.Basis()
	if err != nil || matrix.Len() != 1 || matrix.A[0][0].Cmp(big.NewRat(7, 1)) != 0 {
		t.Fatal("seed", matrix, err)
	}
}

func TestKarrSourceUnionFailureDoesNotPrepareDestination(t *testing.T) {
	source := NewKarrEmpty(1, nil)
	large, ok := new(big.Rat).SetString("9223372036854775808")
	if !ok {
		t.Fatal("numeral")
	}
	if err := source.AddFact([]*big.Rat{large}); err != nil {
		t.Fatal(err)
	}
	target := NewKarrFull(1, nil)
	if _, err := KarrSourceUnion(target, source); !errors.Is(err, hilbert.ErrOverflow) {
		t.Fatal("overflow", err)
	}
	basis, err := target.Basis()
	if err != nil || basis.Len() != 3 {
		t.Fatal("destination changed on source failure", basis, err)
	}
}

func TestKarrSourceUnionInvalidatesUnchangedAndDoesNotRecheckEmpty(t *testing.T) {
	target := NewKarrEmpty(1, nil)
	if err := target.AddFact([]*big.Rat{big.NewRat(7, 1)}); err != nil {
		t.Fatal(err)
	}
	full := NewKarrFull(1, nil)
	changed, err := KarrSourceUnion(target, full)
	if err != nil || changed || target.inequalitiesValid || !target.basisValid {
		t.Fatal("unchanged union cache", changed, err, target)
	}
	impossible := NewKarrFull(1, nil)
	if err = impossible.AddConstraint([]big.Rat{*big.NewRat(1, 1)}, big.NewRat(-1, 1), true); err != nil {
		t.Fatal(err)
	}
	if err = impossible.AddConstraint([]big.Rat{*big.NewRat(1, 1)}, big.NewRat(-2, 1), true); err != nil {
		t.Fatal(err)
	}
	dest := NewKarrEmpty(1, nil)
	changed, err = KarrSourceUnion(dest, impossible)
	if err != nil || !changed || dest.Empty() || !impossible.Empty() || !dest.basisValid || dest.basis.Len() != 0 {
		t.Fatal("lazy inconsistency union", changed, err, dest, impossible)
	}
}
