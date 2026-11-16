package numeral

// Source: src/util/mpf.cpp:667-746.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"math"
	"os"
	"strings"
	"testing"
)

func TestArbitraryFloatDivReachedUndefinedExponentSubtraction(t *testing.T) {
	m, _, _ := mpfNumeralHelpersFixture(t)
	var x, y, out ArbitraryFloat
	n := NewInt(0)
	serviceMust(t, m.SetComponents(&x, 17, 24, true, math.MinInt64, &n))
	serviceMust(t, m.SetComponents(&y, 17, 24, false, 1, &n))
	serviceMust(t, m.MkOne(11, 53, false, &out))
	if e := m.Div(ArbitraryFloatNearestEven, &x, &y, &out); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("reached undefined signed difference", e)
	}
	if out.EBits() != 17 || out.SBits() != 24 || !out.Sign() || out.exponent != 0 || out.significand != NewInt(0) || m.div.roundFrame != nil {
		t.Fatal("publication before undefined expression", out)
	}
	serviceMust(t, m.CleanupDiv())
	serviceMust(t, m.CleanupValue(&out))
	serviceMust(t, m.CleanupValue(&x))
	serviceMust(t, m.CleanupValue(&y))
}

func TestArbitraryFloatDivDebugFormatGuardAndErasedMode(t *testing.T) {
	d, log := bindMPNDebugActions(t, "TC")
	m, _, b := mpfNumeralHelpersFixture(t)
	var x, y, out ArbitraryFloat
	serviceMust(t, m.MkZero(11, 53, false, &x))
	serviceMust(t, m.MkOne(8, 24, true, &y))
	serviceMust(t, m.MkOne(11, 53, false, &out))
	before, used := out, b.Used()
	e := m.Div(ArbitraryFloatRoundingMode(255), &x, &y, &out)
	if modularSourceDebug {
		if !isMPNVerificationThrow(e) || out != before || b.Used() != used || m.div.manager != nil {
			t.Fatal("initial source format assertion", e, out)
		}
		contents, readErr := os.ReadFile(log.Name())
		serviceMust(t, readErr)
		want := "File: src/util/mpf.cpp\nLine: 668\nx.sbits == y.sbits && x.ebits == y.ebits\n"
		if n := strings.Count(string(contents), want); n != 1 {
			t.Fatal("source div format assertion", n, string(contents))
		}
		serviceMust(t, m.Div(ArbitraryFloatRoundingMode(255), &x, &y, &out))
		zero, e := m.IsZero(&out)
		serviceMust(t, e)
		if !zero || !out.Sign() || out.EBits() != 11 || out.SBits() != 53 {
			t.Fatal("Continue follows source zero shortcut", out)
		}
		serviceMust(t, d.EnableAssertions(false))
		serviceMust(t, m.Div(ArbitraryFloatRoundingMode(255), &x, &y, &out))
	} else {
		serviceMust(t, e)
		zero, e := m.IsZero(&out)
		serviceMust(t, e)
		if !zero || !out.Sign() || out.EBits() != 11 || out.SBits() != 53 || m.div.manager != nil {
			t.Fatal("release format and rounding-mode erasure", out)
		}
	}
	if modularSourceDebug {
		serviceMust(t, d.Close())
		if e := m.Div(ArbitraryFloatNearestEven, &y, &y, &out); !errors.Is(e, arena.ErrClosed) {
			t.Fatal("valid div reads closed source assertion owner", e)
		}
	}
	serviceMust(t, m.Del(&out))
	serviceMust(t, m.Del(&x))
	serviceMust(t, m.Del(&y))
}

func TestArbitraryFloatDivGeneralPublicationAndCopiedScope(t *testing.T) {
	m, _, b := mpfNumeralHelpersFixture(t)
	n := NewInt(1)
	serviceMust(t, m.integer.Mul2k(&n, 3998, &n))
	var x, y, out ArbitraryFloat
	serviceMust(t, m.SetComponents(&x, 17, 4000, true, 1, &n))
	serviceMust(t, m.SetComponents(&y, 17, 4000, false, 2, &n))
	serviceMust(t, m.integer.Del(&n))
	serviceMust(t, m.MkOne(11, 53, false, &out))
	_, e := m.IsNormal(&x)
	serviceMust(t, e)
	// Refuse the first allocating local copy after classification and after
	// publication of output format/sign, before output exponent/significand.
	b.FailAfter(0)
	body := m.Div(ArbitraryFloatNearestEven, &x, &y, &out)
	if !numericPhysicalReservationRefusal(body) || !m.div.active {
		t.Fatal("actual initial local copy was not refused", body)
	}
	if out.EBits() != 17 || out.SBits() != 4000 || !out.Sign() || out.exponent != 0 || out.significand != NewInt(0) {
		t.Fatal("literal output publication prefix", out)
	}
	copy := m.div
	copy.hooks.Policy = &copy
	if copy.NumericBound(false) || copy.NumericRoots(m.integer, nil) != nil || numeralContextRef(copy.context()).roots(m.integer) != nil {
		t.Fatal("copied lexical scope acquired allocation authority")
	}
	integerPhysicalAudit(t, m.integer, &x.significand, &out.significand, &m.div.a.significand, &m.div.b.significand, &y.significand)
	b.FailAfter(-1)
	serviceMust(t, m.CleanupDiv())
	serviceMust(t, m.CleanupDiv())
	if numeralContextRef(m.div.context()).roots(m.integer) != nil {
		t.Fatal("ended caller acquired roots")
	}
	serviceMust(t, m.CleanupValue(&out))
	serviceMust(t, m.CleanupValue(&x))
	serviceMust(t, m.CleanupValue(&y))
}
func TestArbitraryFloatDivOverflowDirectedAndSignedZeros(t *testing.T) {
	m, _, _ := mpfNumeralHelpersFixture(t)
	for _, mode := range mpfIntegralModes {
		for _, negative := range []bool{false, true} {
			var x, y, out ArbitraryFloat
			n := NewInt(3)
			serviceMust(t, m.SetComponents(&x, 2, 3, negative, 1, &n)) // +/-3.5
			half := NewInt(2)
			serviceMust(t, m.SetComponents(&y, 2, 3, false, -1, &half)) // Denormal 0.5.
			serviceMust(t, m.Div(mode, &x, &y, &out))                   // Exact +/-7 overflows.
			inf, e := m.IsInf(&out)
			serviceMust(t, e)
			wantInf := mode == ArbitraryFloatNearestEven || mode == ArbitraryFloatNearestAway || mode == ArbitraryFloatTowardPositive && !negative || mode == ArbitraryFloatTowardNegative && negative
			if inf != wantInf || out.Sign() != negative {
				t.Fatal("signed overflow policy", mode, out, inf)
			}
			if !inf {
				actual, e := m.integer.Int64(&out.significand)
				serviceMust(t, e)
				if out.exponent != 1 || actual != 3 {
					t.Fatal("maximum finite on directed overflow", out, actual)
				}
			}
			serviceMust(t, m.MkZero(2, 3, negative, &x))
			for _, other := range []bool{false, true} {
				serviceMust(t, m.MkOne(2, 3, other, &y))
				serviceMust(t, m.Div(mode, &x, &y, &out))
				zero, e := m.IsZero(&out)
				serviceMust(t, e)
				if !zero || out.Sign() != (negative != other) {
					t.Fatal("division signed zero", mode, negative, other, out)
				}
			}
			serviceMust(t, m.Del(&out))
			serviceMust(t, m.Del(&x))
			serviceMust(t, m.Del(&y))
		}
	}
}
