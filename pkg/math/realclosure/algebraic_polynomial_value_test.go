package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestAlgebraicPolynomialValueDegreeAndOwnership(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	root := roots[1].root
	refs := root.refs
	if _, err := f.store.newAlgebraicPolynomialValue(root, []*realValue{one, nil, one}, roots[1].functionInterval); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("unreduced defining-degree numerator accepted", err)
	}
	if root.refs != refs {
		t.Fatal("rejected value changed root ownership")
	}
	value, err := f.store.newAlgebraicPolynomialValue(root, []*realValue{one, one}, roots[1].functionInterval)
	if err != nil || root.refs != refs+1 {
		t.Fatal(root.refs, err)
	}
	if err := f.store.release(value); err != nil || root.refs != refs {
		t.Fatal(root.refs, err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
