package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestExactBinaryFallbackRejectsMalformedPolynomial(t *testing.T) {
	f := newExactBinaryFallbackFixture(t, true)
	before := f.alpha.refs
	for _, p := range [][]*realValue{nil, {f.alpha, nil}} {
		value, err := f.base.store.exactPolynomialValueAtBinaryBound(p, f.alpha.root.owner.intervals, &f.oneBound, true)
		if !errors.Is(err, arena.ErrInvalidSlot) || value != nil {
			t.Fatal(value, err)
		}
	}
	if f.alpha.refs != before || f.alpha.deleted {
		t.Fatal("failure consumed coefficient", before, f.alpha.refs)
	}
}
