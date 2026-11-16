package numeral

// Source: src/util/mpf.cpp:1047-1170; src/util/mpf.h:133.

import (
	"math"
	"testing"
)

func TestArbitraryFloatRoundToIntegralInvalidModeSourceBranches(t *testing.T) {
	if modularSourceDebug {
		bindMPNDebugActions(t, "TTTTT")
	}
	m, _, _ := mpfNumeralHelpersFixture(t)
	invalid := ArbitraryFloatRoundingMode(255)
	for _, input := range []float64{0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN(), .25, .5, .75, -.5, 1, 1.5, -1.5, 2.5} {
		var x, out ArbitraryFloat
		serviceMust(t, m.SetDouble(&x, 11, 53, input))
		serviceMust(t, m.SetDouble(&out, 11, 53, 9))
		before := out
		body := m.RoundToIntegral(invalid, &x, &out)
		if modularSourceDebug && input != 0 && math.Abs(input) < 1 {
			if !isMPNVerificationThrow(body) || out != before {
				t.Fatal("reached scalar rounding-mode assertion", input, body, out)
			}
		} else {
			serviceMust(t, body)
			actual, e := m.ToDouble(&out)
			serviceMust(t, e)
			want := math.Trunc(input)
			if input != 0 && math.Abs(input) < 1 {
				want = math.Copysign(0, input)
				if math.Abs(input) >= .5 {
					want = math.Copysign(1, input)
				}
			}
			if math.IsNaN(want) {
				if !math.IsNaN(actual) {
					t.Fatal("erased invalid mode for NaN", actual)
				}
			} else if math.Float64bits(actual) != math.Float64bits(want) {
				t.Fatal("invalid mode source default", input, actual, want)
			}
		}
		serviceMust(t, m.Del(&out))
		serviceMust(t, m.Del(&x))
	}
	if modularSourceDebug {
		var input, out ArbitraryFloat
		serviceMust(t, m.MkOne(11, 53, true, &out))
		before := out
		if e := m.RoundToIntegral(ArbitraryFloatNearestEven, &input, &out); !isMPNVerificationThrow(e) || out != before {
			t.Fatal("initial format assertion", e, out)
		}
		serviceMust(t, m.Del(&out))
	}
}

func TestArbitraryFloatRoundToIntegralGeneralPublicationAndCopiedScope(t *testing.T) {
	m, _, b := mpfNumeralHelpersFixture(t)
	n := NewInt(1)
	serviceMust(t, m.integer.Mul2k(&n, 3998, &n))
	var x, out ArbitraryFloat
	serviceMust(t, m.SetComponents(&x, 17, 4000, true, 1, &n))
	serviceMust(t, m.integer.Del(&n))
	serviceMust(t, m.MkOne(11, 53, false, &out))
	_, e := m.IsNormal(&x)
	serviceMust(t, e)
	// Refuse the first allocating local copy after classification and after
	// publication of output format/sign, before output exponent/significand.
	b.FailAfter(0)
	body := m.RoundToIntegral(ArbitraryFloatNearestEven, &x, &out)
	if !numericPhysicalReservationRefusal(body) || !m.integralRound.active {
		t.Fatal("actual initial local copy was not refused", body)
	}
	if out.EBits() != 17 || out.SBits() != 4000 || !out.Sign() || out.exponent != 0 || out.significand != NewInt(0) {
		t.Fatal("literal output publication prefix", out)
	}
	copy := m.integralRound
	copy.hooks.Policy = &copy
	if copy.NumericBound(false) || copy.NumericRoots(m.integer, nil) != nil || numeralContextRef(copy.context()).roots(m.integer) != nil {
		t.Fatal("copied lexical scope acquired allocation authority")
	}
	integerPhysicalAudit(t, m.integer, &x.significand, &out.significand, &m.integralRound.a.significand)
	b.FailAfter(-1)
	serviceMust(t, m.CleanupRoundToIntegral())
	serviceMust(t, m.CleanupRoundToIntegral())
	if numeralContextRef(m.integralRound.context()).roots(m.integer) != nil {
		t.Fatal("ended caller acquired roots")
	}
	serviceMust(t, m.CleanupValue(&out))
	serviceMust(t, m.CleanupValue(&x))
}

func TestArbitraryFloatRoundToIntegralCarryReachesSpecialExponent(t *testing.T) {
	m, _, _ := mpfNumeralHelpersFixture(t)
	for _, sign := range []bool{false, true} {
		for _, mode := range mpfIntegralModes {
			var x, out ArbitraryFloat
			sig := NewInt(3)
			serviceMust(t, m.SetComponents(&x, 2, 3, sign, 1, &sig)) // +/-3.5
			serviceMust(t, m.RoundToIntegral(mode, &x, &out))
			inf, e := m.IsInf(&out)
			serviceMust(t, e)
			wantInf := mode == ArbitraryFloatNearestEven || mode == ArbitraryFloatNearestAway || mode == ArbitraryFloatTowardPositive && !sign || mode == ArbitraryFloatTowardNegative && sign
			if inf != wantInf || out.Sign() != sign {
				t.Fatal("source carry into top exponent", sign, mode, out, inf, wantInf)
			}
			// Source integral rounding has no overflow-policy call: carrying
			// directly into top exponent and stripping the hidden bit yields inf.
			wantExponent, wantSig := int64(1), int64(2)
			if wantInf {
				wantExponent, wantSig = 2, 0
			}
			actualSig, e := m.integer.Int64(&out.significand)
			serviceMust(t, e)
			if out.exponent != wantExponent || actualSig != wantSig {
				t.Fatal("literal renormalization", out, actualSig)
			}
			serviceMust(t, m.Del(&out))
			serviceMust(t, m.Del(&x))
		}
	}
}
