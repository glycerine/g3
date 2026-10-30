package hilbert

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOrderedSaturationSourceExample(t *testing.T) {
	b := New(nil)
	if err := b.AddEQ(rationals(1, 1, -2), rational(0)); err != nil {
		t.Fatal(err)
	}
	if err := b.AddEQ(rationals(1, 0, -1), rational(0)); err != nil {
		t.Fatal(err)
	}
	if err := b.AddLE(rationals(0, 1, -1), rational(0)); err != nil {
		t.Fatal(err)
	}
	got, err := b.Saturate()
	if err != nil || got != lbool.True {
		t.Fatalf("Saturate = %v, %v", got, err)
	}
	validateBasis(t, b)
}
