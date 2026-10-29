package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestRetainedAlgebraicMultiplicationReduction(t *testing.T) {
	f := newBoundedSignFixture(t, -3, 3)
	minusTwo, one, minusOne := f.rational(-2), f.rational(1), f.rational(-1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	alpha := roots[1]
	square, err := f.store.multiplyRetainedValues(alpha, alpha)
	if err != nil || square == nil || !square.isRational {
		t.Fatal(square, err)
	}
	expectRationalPolynomial(t, f, []*realValue{square}, 2)
	plusOne, err := f.store.addRetainedValues(alpha, one)
	if err != nil {
		t.Fatal(err)
	}
	minusOneValue, err := f.store.addRetainedValues(alpha, minusOne)
	if err != nil {
		t.Fatal(err)
	}
	product, err := f.store.multiplyRetainedValues(plusOne, minusOneValue)
	if err != nil || product == nil || !product.isRational {
		t.Fatal(product, err)
	}
	expectRationalPolynomial(t, f, []*realValue{product}, 1)
	for _, value := range []*realValue{square, plusOne, minusOneValue, product} {
		if err := f.store.release(value); err != nil {
			t.Fatal(err)
		}
	}
	releaseIsolatedValues(t, f.store, roots)
}

func TestRetainedMultiplicationRejectsDifferentExtensions(t *testing.T) {
	f := newBoundedSignFixture(t, -3, 3)
	one, minusTwo, minusThree := f.rational(1), f.rational(-2), f.rational(-3)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	squareRoots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	cubeRoots, err := owner.isolateRationalRoots([]*realValue{minusThree, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.multiplyRetainedValues(squareRoots[1], cubeRoots[1]); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, squareRoots)
	releaseIsolatedValues(t, f.store, cubeRoots)
}
