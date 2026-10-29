package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestRetainedAffineAlgebraicNegAddSubtract(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil || len(roots) != 2 {
		t.Fatal(len(roots), err)
	}
	alpha := roots[1]
	negative, err := f.store.negateRetainedValue(alpha)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, negative.functionNumerator.coefficients, 0, -1)
	sign, err := f.store.signValue(negative)
	if err != nil || sign != -1 {
		t.Fatal(sign, err)
	}
	alphaPlusOne, err := f.store.addRetainedValues(alpha, one)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, alphaPlusOne.functionNumerator.coefficients, 1, 1)
	if alphaPlusOne.root != alpha.root || alphaPlusOne.root.refs != 3 {
		t.Fatal("sum did not retain the algebraic extension")
	}
	twoAlpha, err := f.store.addRetainedValues(alpha, alpha)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, twoAlpha.functionNumerator.coefficients, 0, 2)
	zero, err := f.store.subtractRetainedValues(alpha, alpha)
	if err != nil || zero != nil {
		t.Fatal("self subtraction", zero, err)
	}
	for _, value := range []*realValue{negative, alphaPlusOne, twoAlpha} {
		if err := f.store.release(value); err != nil {
			t.Fatal(err)
		}
	}
	releaseIsolatedValues(t, f.store, roots)
}

func TestRetainedAffineArithmeticRationalAndExtensionGuards(t *testing.T) {
	f := newBoundedSignFixture(t, -3, 3)
	one, two, minusTwo := f.rational(1), f.rational(2), f.rational(-2)
	sum, err := f.store.addRetainedValues(one, two)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, []*realValue{sum}, 3)
	if err := f.store.release(sum); err != nil {
		t.Fatal(err)
	}
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	squareRoots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	minusThree := f.rational(-3)
	cubeRoots, err := owner.isolateRationalRoots([]*realValue{minusThree, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.addRetainedValues(squareRoots[1], cubeRoots[1]); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("different extensions accepted", err)
	}
	releaseIsolatedValues(t, f.store, squareRoots)
	releaseIsolatedValues(t, f.store, cubeRoots)
}
