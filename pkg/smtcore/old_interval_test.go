package smtcore

import (
	"testing"
)

func TestSMTOldIntervalCloneOwnsRationalEndpoints(t *testing.T) {
	var manager smtArithmeticBoundDependencyManager
	lower := &smtArithmeticBound{variable: 1}
	upper := &smtArithmeticBound{variable: 2}
	original, err := newSMTOldIntervalBounds(&manager, testQ(), oiRat(2, 3), false, manager.Leaf(lower), oiRat(7, 5), true, manager.Leaf(upper))
	astTestMust(t, err)
	// Exercise intervals produced by arithmetic, whose big.Rat endpoints
	// can retain spare limb capacity, rather than only fresh numerals.
	unit, err := newSMTOldIntervalPoint(&manager, testQ(), oiR(1), nil, nil)
	astTestMust(t, err)
	astTestMust(t, original.add(unit))
	astTestMust(t, original.sub(unit))
	copy := oiClone(original)
	point, err := newSMTOldIntervalPoint(&manager, testQ(), oiRat(11, 13), nil, nil)
	astTestMust(t, err)
	astTestMust(t, copy.add(point))
	if original.String() != "[2/3, 7/5)" || original.lowerDep != copy.lowerDep || original.upperDep != copy.upperDep {
		t.Fatal("modifying a copied interval changed its source", original, copy)
	}
	if copy.String() != "[59/39, 146/65)" {
		t.Fatal("copied interval sum", copy)
	}
	// Mutations must be independent in both directions, while explanation
	// nodes stay shared immutable leaves owned by the dependency manager.
	astTestMust(t, testQ().SetInt64(&original.lower.value, 17))
	astTestMust(t, testQ().SetInt64(&original.upper.value, 19))
	if copy.String() != "[59/39, 146/65)" || !manager.Contains(copy.lowerDep, lower) || !manager.Contains(copy.upperDep, upper) {
		t.Fatal("source mutation changed copied interval or premises", copy)
	}
}

func TestSMTOldIntervalAddSubtractContainsAndDependencies(t *testing.T) {
	var manager smtArithmeticBoundDependencyManager
	a, b := &smtArithmeticBound{variable: 1}, &smtArithmeticBound{variable: 2}
	left, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(-1), false, manager.Leaf(a), oiR(2), true, manager.Leaf(b))
	if err != nil {
		t.Fatal(err)
	}
	right, err := newSMTOldIntervalPoint(&manager, testQ(), oiR(3), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = left.add(right); err != nil || left.String() != "[2, 5)" {
		t.Fatalf("addition = %s, %v", left, err)
	}
	if !left.testContains(oiR(2)) || left.testContains(oiR(5)) || left.containsZero() {
		t.Fatal("containment after addition")
	}
	if !manager.Contains(left.lowerDep, a) || !manager.Contains(left.upperDep, b) {
		t.Fatal("bound dependencies lost")
	}
	if err = left.sub(right); err != nil || left.String() != "[-1, 2)" {
		t.Fatalf("subtraction = %s, %v", left, err)
	}
}

func TestSMTOldIntervalInverseAndPower(t *testing.T) {
	var manager smtArithmeticBoundDependencyManager
	positive, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(0), true, nil, oiR(2), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = positive.inverse(); err != nil || positive.String() != "[1/2, oo)" {
		t.Fatalf("inverse = %s, %v", positive, err)
	}
	crossing, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(-3), false, nil, oiR(2), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	crossing.power(2)
	if crossing.String() != "[0, 9]" {
		t.Fatalf("even power = %s", crossing)
	}
	negative, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(-2), false, nil, oiR(-1), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	negative.power(3)
	if negative.String() != "[-8, -1]" {
		t.Fatalf("odd power = %s", negative)
	}
	zero, err := newSMTOldIntervalPoint(&manager, testQ(), oiR(0), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = zero.inverse(); err == nil {
		t.Fatal("inverse accepted interval containing zero")
	}
}

func TestSMTOldIntervalMultiplicationSignTable(t *testing.T) {
	var manager smtArithmeticBoundDependencyManager
	makeInterval := func(lower, upper int64) *smtOldInterval {
		result, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(lower), false, nil, oiR(upper), false, nil)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	classes := []*smtOldInterval{makeInterval(-3, -2), makeInterval(-2, 3), makeInterval(2, 4)}
	wants := [3][3]string{{"[4, 9]", "[-9, 6]", "[-12, -4]"}, {"[-9, 6]", "[-6, 9]", "[-8, 12]"}, {"[-12, -4]", "[-8, 12]", "[4, 16]"}}
	for left := range classes {
		for right := range classes {
			result := oiClone(classes[left])
			if err := result.mul(classes[right]); err != nil {
				t.Fatal(left, right, err)
			}
			if result.String() != wants[left][right] {
				t.Fatalf("case %d,%d = %s, want %s", left, right, result, wants[left][right])
			}
		}
	}
}

func TestSMTOldIntervalMultiplyOpenZeroAndDivide(t *testing.T) {
	var manager smtArithmeticBoundDependencyManager
	left, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(0), true, nil, oiR(2), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	right, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(-1), false, nil, oiR(0), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = left.mul(right); err != nil || left.String() != "[-2, 0]" {
		t.Fatalf("open-zero product = %s, %v", left, err)
	}
	numerator, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(2), false, nil, oiR(4), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	denominator, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(1), false, nil, oiR(2), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = numerator.div(denominator); err != nil || numerator.String() != "[1, 4]" {
		t.Fatalf("division = %s, %v", numerator, err)
	}
	zero, err := newSMTOldIntervalPoint(&manager, testQ(), oiR(0), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = zero.div(right); err == nil {
		t.Fatal("division by interval containing zero accepted")
	}
}

func TestSMTOldIntervalMultiplicationClosedOracle(t *testing.T) {
	var manager smtArithmeticBoundDependencyManager
	for a := int64(-5); a <= 5; a++ {
		for b := a; b <= 5; b++ {
			for c := int64(-5); c <= 5; c++ {
				for d := c; d <= 5; d++ {
					left, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(a), false, nil, oiR(b), false, nil)
					if err != nil {
						t.Fatal(err)
					}
					right, err := newSMTOldIntervalBounds(&manager, testQ(), oiR(c), false, nil, oiR(d), false, nil)
					if err != nil {
						t.Fatal(err)
					}
					products := []int64{a * c, a * d, b * c, b * d}
					minimum, maximum := products[0], products[0]
					for _, value := range products[1:] {
						if value < minimum {
							minimum = value
						}
						if value > maximum {
							maximum = value
						}
					}
					if err = left.mul(right); err != nil {
						t.Fatal(a, b, c, d, err)
					}
					if left.lower.infinite() || left.upper.infinite() || !oiIs(&left.lower.value, minimum, 1) || !oiIs(&left.upper.value, maximum, 1) || left.lowerOpen || left.upperOpen {
						t.Fatalf("[%d,%d]*[%d,%d] = %s, want [%d, %d]", a, b, c, d, left, minimum, maximum)
					}
				}
			}
		}
	}
}
