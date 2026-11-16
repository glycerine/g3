package numeral

// Source: src/util/mpf.cpp:1019-1025.

import "testing"

func TestArbitraryFloatSqrtGeneralPublicationAndCopiedScope(t *testing.T) {
	m, _, b := mpfNumeralHelpersFixture(t)
	n := NewInt(1)
	serviceMust(t, m.integer.Mul2k(&n, 118, &n))
	var x, out ArbitraryFloat
	serviceMust(t, m.SetComponents(&x, 17, 120, false, 1, &n))
	serviceMust(t, m.integer.Del(&n))
	serviceMust(t, m.MkOne(11, 53, false, &out))
	_, e := m.IsNormal(&x)
	serviceMust(t, e)
	// Refuse the first allocating local copy after classification and after
	// publication of output format/sign, before output exponent/significand.
	b.FailAfter(0)
	body := m.Sqrt(ArbitraryFloatNearestEven, &x, &out)
	if !numericPhysicalReservationRefusal(body) || !m.sqrt.active {
		t.Fatal("actual initial local copy was not refused", body)
	}
	if out.EBits() != 17 || out.SBits() != 120 || out.Sign() || out.exponent != 0 || out.significand != NewInt(0) {
		t.Fatal("literal output publication prefix", out)
	}
	copy := m.sqrt
	copy.hooks.Policy = &copy
	if copy.NumericBound(false) || copy.NumericRoots(m.integer, nil) != nil || numeralContextRef(copy.context()).roots(m.integer) != nil {
		t.Fatal("copied lexical scope acquired allocation authority")
	}
	zero, sentinel := NewInt(0), NewInt(7)
	used, generation := b.Used(), m.integer.rootGeneration
	if _, e := m.integer.rootWithAllocationContext(&zero, 2, &sentinel, nil, nil, numeralContextRef(copy.context())); e != ErrNumericOwner || sentinel != NewInt(7) || b.Used() != used || m.integer.rootGeneration != generation {
		t.Fatal("copied caller mutated scalar output or entered root", e, sentinel)
	}
	integerPhysicalAudit(t, m.integer, &x.significand, &out.significand, &m.sqrt.a.significand)
	b.FailAfter(-1)
	serviceMust(t, m.CleanupSqrt())
	serviceMust(t, m.CleanupSqrt())
	if numeralContextRef(m.sqrt.context()).roots(m.integer) != nil {
		t.Fatal("ended caller acquired roots")
	}
	used, generation = b.Used(), m.integer.rootGeneration
	if _, e := m.integer.rootWithAllocationContext(&zero, 2, &sentinel, nil, nil, numeralContextRef(m.sqrt.context())); e != ErrNumericOwner || sentinel != NewInt(7) || b.Used() != used || m.integer.rootGeneration != generation {
		t.Fatal("ended caller mutated scalar output or entered root", e, sentinel)
	}
	serviceMust(t, m.CleanupValue(&out))
	serviceMust(t, m.CleanupValue(&x))
}
