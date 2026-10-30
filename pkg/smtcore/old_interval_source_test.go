package smtcore

// Source: src/smt/old_interval.cpp:20-151; src/test/old_interval.cpp:20-115.

import (
	"math/big"
	"testing"
)

func TestSMTOldIntervalRegisteredExtendedNumeralAssertions(t *testing.T) {
	finite := func(n int64) smtExtendedRational { return smtFiniteExtended(big.NewRat(n, 1)) }
	positiveInfinity := smtInfiniteExtended(true)
	negativeInfinity := smtInfiniteExtended(false)
	zero := finite(0)
	type binaryCase struct {
		name              string
		left, right, want smtExtendedRational
	}
	for _, tc := range []binaryCase{
		{"add finite", finite(10), finite(3), finite(13)},
		{"add plus zero", positiveInfinity, zero, positiveInfinity},
		{"add minus zero", negativeInfinity, zero, negativeInfinity},
		{"add minus finite positive", negativeInfinity, finite(3), negativeInfinity},
		{"add plus plus", positiveInfinity, positiveInfinity, positiveInfinity},
		{"add minus minus", negativeInfinity, negativeInfinity, negativeInfinity},
		{"add minus finite positive ten", negativeInfinity, finite(10), negativeInfinity},
		{"add minus finite negative ten", negativeInfinity, finite(-10), negativeInfinity},
		{"add plus finite positive ten", positiveInfinity, finite(10), positiveInfinity},
		{"add plus finite negative ten", positiveInfinity, finite(-10), positiveInfinity},
		{"subtract finite", finite(10), finite(3), finite(7)},
		{"subtract plus zero", positiveInfinity, zero, positiveInfinity},
		{"subtract minus zero", negativeInfinity, zero, negativeInfinity},
		{"subtract minus finite", negativeInfinity, finite(3), negativeInfinity},
		{"subtract plus minus", positiveInfinity, negativeInfinity, positiveInfinity},
		{"subtract minus plus", negativeInfinity, positiveInfinity, negativeInfinity},
		{"subtract zero minus", zero, negativeInfinity, positiveInfinity},
		{"subtract zero plus", zero, positiveInfinity, negativeInfinity},
		{"subtract negative finite minus", finite(-10), negativeInfinity, positiveInfinity},
		{"subtract positive finite minus", finite(10), negativeInfinity, positiveInfinity},
		{"subtract negative finite plus", finite(-10), positiveInfinity, negativeInfinity},
		{"subtract positive finite plus", finite(10), positiveInfinity, negativeInfinity},
		{"multiply positive plus", finite(10), positiveInfinity, positiveInfinity},
		{"multiply negative plus", finite(-10), positiveInfinity, negativeInfinity},
		{"multiply zero plus", zero, positiveInfinity, zero},
		{"multiply zero minus", zero, negativeInfinity, zero},
		{"multiply zero finite", zero, finite(10), zero},
		{"multiply finite", finite(10), finite(-20), finite(-200)},
		{"multiply finite small", finite(3), finite(2), finite(6)},
		{"multiply plus plus", positiveInfinity, positiveInfinity, positiveInfinity},
		{"multiply plus minus", positiveInfinity, negativeInfinity, negativeInfinity},
		{"multiply minus minus", negativeInfinity, negativeInfinity, positiveInfinity},
		{"multiply minus plus", negativeInfinity, positiveInfinity, negativeInfinity},
		{"multiply minus positive", negativeInfinity, finite(10), negativeInfinity},
		{"multiply minus negative", negativeInfinity, finite(-10), positiveInfinity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.left.clone()
			var err error
			switch tc.name[:3] {
			case "add":
				err = got.add(&tc.right)
			case "sub":
				err = got.sub(&tc.right)
			case "mul":
				got.mul(&tc.right)
			}
			if err != nil || !got.equal(&tc.want) {
				t.Fatalf("result=%s want=%s err=%v", &got, &tc.want, err)
			}
		})
	}

	// Source comparisons cover infinity ordering, finite ordering, equality and
	// all four derived relations at both strict and equal boundaries.
	ordered := []smtExtendedRational{negativeInfinity, finite(-10), finite(-3), zero, finite(2), finite(4), finite(10), finite(30), positiveInfinity}
	for i, left := range ordered {
		for j, right := range ordered {
			got := left.cmp(&right)
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if got != want || left.equal(&right) != (i == j) {
				t.Fatalf("source ordering %s vs %s: cmp=%d want=%d", &left, &right, got, want)
			}
		}
	}

	value := finite(10)
	value.neg()
	wantNegativeTen := finite(-10)
	if !value.equal(&wantNegativeTen) {
		t.Fatal("finite negation", &value)
	}
	value = positiveInfinity.clone()
	value.neg()
	if !value.equal(&negativeInfinity) {
		t.Fatal("positive infinity negation", &value)
	}
	value.neg()
	if !value.equal(&positiveInfinity) {
		t.Fatal("negative infinity negation", &value)
	}
	negativeTen, positiveTen := finite(-10), finite(10)
	if negativeInfinity.sign() >= 0 || zero.sign() != 0 || positiveInfinity.sign() <= 0 || negativeTen.sign() >= 0 || positiveTen.sign() <= 0 {
		t.Fatal("source sign predicate")
	}
	if !negativeInfinity.infinite() || !positiveInfinity.infinite() || zero.infinite() || positiveTen.infinite() || negativeInfinity.zero() || positiveInfinity.zero() || !zero.zero() || positiveTen.zero() {
		t.Fatal("source infinite/zero predicates")
	}
}

func TestSMTOldIntervalRegisteredSourceDiagnosticArithmetic(t *testing.T) {
	var manager smtArithmeticBoundDependencyManager
	makeInterval := func(lower, upper int64, lowerOpen, upperOpen bool) *smtOldInterval {
		result, err := newSMTOldIntervalBounds(&manager, big.NewRat(lower, 1), lowerOpen, nil, big.NewRat(upper, 1), upperOpen, nil)
		astTestMust(t, err)
		return result
	}
	all, err := newSMTOldInterval(&manager)
	astTestMust(t, err)
	fixtures := []struct {
		name   string
		value  *smtOldInterval
		square bool
		want   string
	}{
		{"singleton", makeInterval(10, 10, false, false), false, "[10, 10]"},
		{"all", all, false, "(-oo, oo)"},
		{"lower", func() *smtOldInterval {
			x, _ := newSMTOldIntervalHalfBound(&manager, big.NewRat(-10, 1), false, true, nil)
			return x
		}(), false, "[-10, oo)"},
		{"upper", func() *smtOldInterval {
			x, _ := newSMTOldIntervalHalfBound(&manager, big.NewRat(10, 1), false, false, nil)
			return x
		}(), false, "(-oo, 10]"},
		{"lower open", func() *smtOldInterval {
			x, _ := newSMTOldIntervalHalfBound(&manager, big.NewRat(-10, 1), true, true, nil)
			return x
		}(), false, "(-10, oo)"},
		{"upper open", func() *smtOldInterval {
			x, _ := newSMTOldIntervalHalfBound(&manager, big.NewRat(10, 1), true, false, nil)
			return x
		}(), false, "(-oo, 10)"},
		{"bounded", makeInterval(2, 10, false, false), false, "[2, 10]"},
		{"square crossing zero", makeInterval(-2, 3, true, false), true, "[0, 9]"},
		{"square open negative endpoint", makeInterval(-4, 3, true, false), true, "[0, 16)"},
		{"square positive interval", makeInterval(2, 4, true, false), true, "(4, 16]"},
		{"square open zero", makeInterval(0, 3, true, false), true, "[0, 9]"},
		{"square negative interval", makeInterval(-4, -2, true, false), true, "[4, 16)"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			value := fixture.value.clone()
			if fixture.square {
				value.power(2)
			}
			if got := value.String(); got != fixture.want {
				t.Fatalf("source interval=%s want=%s", got, fixture.want)
			}
		})
	}

	// interval_tester::bugs() reproduces the source's zero-times-unbounded
	// sign case, then the registered driver exercises each multiply/divide path.
	bug := func() *smtOldInterval {
		left, err := newSMTOldIntervalHalfBound(&manager, big.NewRat(0, 1), false, true, nil)
		astTestMust(t, err)
		right := makeInterval(-1, 0, false, false)
		astTestMust(t, left.mul(right))
		return left
	}()
	if got := bug.String(); got != "(-oo, 0]" {
		t.Fatal("source bugs() reproduction", got)
	}

	checks := []struct {
		name        string
		left, right *smtOldInterval
		divide      bool
		want        string
	}{
		{"dependency product", makeInterval(-5, 5, true, false), makeInterval(-5, 5, false, true), false, "[-25, 25)"},
		{"bounded over positive halfline", makeInterval(2, 10, false, false), func() *smtOldInterval {
			x, _ := newSMTOldIntervalHalfBound(&manager, big.NewRat(10, 1), false, true, nil)
			return x
		}(), true, "(0, 1]"},
		{"positive halfline division", func() *smtOldInterval {
			x, _ := newSMTOldIntervalHalfBound(&manager, big.NewRat(2, 1), false, true, nil)
			return x
		}(), makeInterval(2, 6, false, false), true, "[1/3, oo)"},
		{"negative interval product", makeInterval(-2, -1, false, true), makeInterval(-3, 0, false, false), false, "[0, 6]"},
		{"positive interval product", makeInterval(1, 2, true, false), makeInterval(0, 3, false, false), false, "[0, 6]"},
		{"mixed open product", makeInterval(1, 2, true, true), makeInterval(-3, 0, false, false), false, "(-6, 0]"},
		{"division by open-zero interval", makeInterval(10, 20, false, false), makeInterval(0, 1, true, false), true, "[10, oo)"},
		{"division by wider open-zero interval", makeInterval(10, 20, false, false), makeInterval(0, 2, true, false), true, "[5, oo)"},
	}
	positiveHalfline, _ := newSMTOldIntervalHalfBound(&manager, big.NewRat(10, 1), false, true, nil)
	astTestMust(t, positiveHalfline.inverse())
	if got := positiveHalfline.String(); got != "(0, 1/10]" {
		t.Fatal("source l(10).inv()", got)
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			result := check.left.clone()
			if check.divide {
				astTestMust(t, result.div(check.right))
			} else {
				astTestMust(t, result.mul(check.right))
			}
			if got := result.String(); got != check.want {
				t.Fatalf("source arithmetic=%s want=%s", got, check.want)
			}
		})
	}
}
