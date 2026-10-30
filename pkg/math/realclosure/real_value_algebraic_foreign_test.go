package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestRealValueAlgebraicProductRejectsDifferentExtensions(t *testing.T) {
	f := newRealFunctionFixture(t)
	otherRoot, err := f.owner.mkAlgebraic([]*realValue{f.minusTwo, nil, f.one}, f.enclosure, f.enclosure, nil, -1)
	mustRealFunction(t, err)
	other, err := f.store.newAlgebraicValue(otherRoot, f.one)
	mustRealFunction(t, err)
	t.Cleanup(func() { mustRealFunction(t, f.store.release(other)) })
	if _, err = f.store.multiplyNormalizedValues(f.alpha, other); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("different algebraic extensions multiplied", err)
	}
}
