package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestRankedFieldValueDivisionAndFailureOwnership(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			quotient, err := f.base.store.divideRankedRetainedValues(f.alpha, f.gamma, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			equal, err := f.base.store.structurallyEqualValues(quotient, f.gamma)
			if err != nil || !equal {
				t.Fatal("alpha/gamma did not reduce to gamma", equal, err)
			}
			before := f.gamma.refs
			if value, err := f.base.store.divideRankedRetainedValues(f.gamma, nil, usePrem); !errors.Is(err, arena.ErrInvalidSlot) || value != nil {
				t.Fatal(value, err)
			}
			if f.gamma.refs != before {
				t.Fatal("failed division changed input ownership", before, f.gamma.refs)
			}
			f.releaseValues(quotient)
		})
	}
}
