package numeral

// Source: src/util/mpf.cpp:480-588.

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestArbitraryFloatAddSubDebugFormatGuardAndErasedMode(t *testing.T) {
	d, log := bindMPNDebugActions(t, "TC")
	m, _, b := mpfNumeralHelpersFixture(t)
	var x, y, out ArbitraryFloat
	serviceMust(t, m.MkZero(11, 53, false, &x))
	serviceMust(t, m.MkOne(8, 24, true, &y))
	serviceMust(t, m.MkOne(11, 53, false, &out))
	before, used := out, b.Used()
	// The debug format assertion precedes even the zero classification. In
	// release the zero shortcut copies y and never consults the rounding mode.
	e := m.Add(ArbitraryFloatRoundingMode(255), &x, &y, &out)
	if modularSourceDebug {
		if !isMPNVerificationThrow(e) || out != before || b.Used() != used || m.addSub.manager != nil {
			t.Fatal("initial format assertion", e, out)
		}
		contents, readErr := os.ReadFile(log.Name())
		serviceMust(t, readErr)
		want := "File: src/util/mpf.cpp\nLine: 479\nx.sbits == y.sbits && x.ebits == y.ebits\n"
		if n := strings.Count(string(contents), want); n != 1 {
			t.Fatal("source add/sub format assertion", n, string(contents))
		}
		e = m.Add(ArbitraryFloatRoundingMode(255), &x, &y, &out)
		serviceMust(t, e)
		if out != y {
			t.Fatal("Continue follows source zero shortcut", out, y)
		}
		serviceMust(t, d.EnableAssertions(false))
		e = m.Add(ArbitraryFloatRoundingMode(255), &x, &y, &out)
		serviceMust(t, e)
		if out != y {
			t.Fatal("disabled format assertion", out, y)
		}
	} else {
		serviceMust(t, e)
		if out != y || m.addSub.manager != nil {
			t.Fatal("release format and mode erasure", out, y)
		}
	}
	if modularSourceDebug {
		serviceMust(t, d.Close())
		if e := m.Add(ArbitraryFloatNearestEven, &y, &y, &out); !errors.Is(e, arena.ErrClosed) {
			t.Fatal("valid add reads closed source assertion owner", e)
		}
	}
	serviceMust(t, m.Del(&out))
	serviceMust(t, m.Del(&x))
	serviceMust(t, m.Del(&y))
}

func TestArbitraryFloatAddSubGeneralPublicationAndCopiedScope(t *testing.T) {
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
	// publication of output format, before output exponent/significand.
	b.FailAfter(0)
	body := m.Add(ArbitraryFloatNearestEven, &x, &y, &out)
	if !numericPhysicalReservationRefusal(body) || !m.addSub.active {
		t.Fatal("actual initial local copy was not refused", body)
	}
	if out.EBits() != 17 || out.SBits() != 4000 || out.Sign() || out.exponent != 0 || out.significand != NewInt(0) {
		t.Fatal("literal output publication prefix", out)
	}
	copy := m.addSub
	copy.hooks.Policy = &copy
	if copy.NumericBound(false) || copy.NumericRoots(m.integer, nil) != nil || numeralContextRef(copy.context()).roots(m.integer) != nil {
		t.Fatal("copied lexical scope acquired allocation authority")
	}
	integerPhysicalAudit(t, m.integer, &x.significand, &out.significand, &m.addSub.a.significand, &m.addSub.b.significand, &y.significand)
	b.FailAfter(-1)
	serviceMust(t, m.CleanupAddSub())
	serviceMust(t, m.CleanupAddSub())
	if numeralContextRef(m.addSub.context()).roots(m.integer) != nil {
		t.Fatal("ended caller acquired roots")
	}
	serviceMust(t, m.CleanupValue(&out))
	serviceMust(t, m.CleanupValue(&x))
	serviceMust(t, m.CleanupValue(&y))
}
