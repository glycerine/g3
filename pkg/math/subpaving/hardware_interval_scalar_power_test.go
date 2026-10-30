package subpaving

import (
	"math"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func hardwareIntervalFixture(t *testing.T) (*numeral.HardwareFloatNumeralManager, *HardwareInterval, *HardwareInterval, *[3]numeral.HardwareFloat) {
	t.Helper()
	m, e := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
	if e != nil {
		t.Fatal(e)
	}
	a, e := NewHardwareInterval(m)
	if e != nil {
		t.Fatal(e)
	}
	out, e := NewHardwareInterval(m)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := out.Close(); e != nil {
			t.Error(e)
		}
		if e := a.Close(); e != nil {
			t.Error(e)
		}
	})
	return m, a, out, &[3]numeral.HardwareFloat{}
}
func TestSubpavingHardwareIntervalScalarMulDivDirectedAndAlias(t *testing.T) {
	m, a, out, values := hardwareIntervalFixture(t)
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	set := func(l, u int32, lop, uop bool) {
		t.Helper()
		must(m.SetInt32(&values[0], l))
		must(m.SetInt32(&values[1], u))
		must(a.SetFinite(&values[0], &values[1], lop, uop))
	}
	check := func(v *HardwareInterval, l, u float64, li, ui, lop, uop bool) {
		t.Helper()
		lo, hi, e := v.endpoints()
		must(e)
		if v.lowerInf != li || v.upperInf != ui || v.lowerOpen != lop || v.upperOpen != uop || !li && m.ToFloat64(lo) != l || !ui && m.ToFloat64(hi) != u {
			t.Fatalf("scalar result [%v,%v] inf=(%t,%t) open=(%t,%t)", m.ToFloat64(lo), m.ToFloat64(hi), v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen)
		}
	}
	k := &values[2]
	set(-1, 2, true, false)
	must(m.SetInt32(k, -2))
	must(out.MulScalar(k, a))
	check(out, -4, 2, false, false, false, true)
	must(a.MulScalar(k, a))
	check(a, -4, 2, false, false, false, true)
	must(m.SetInt32(k, 2))
	must(out.DivScalar(a, k))
	check(out, -2, 1, false, false, false, true)
	must(a.ResetLower())
	must(out.MulScalar(k, a))
	check(out, 0, 4, true, false, true, true)
	must(m.SetInt32(k, -2))
	must(out.DivScalar(a, k))
	check(out, -1, 0, false, true, true, true)
	must(m.Reset(k))
	must(out.MulScalar(k, a))
	check(out, 0, 0, true, true, true, true)
	must(out.DivScalar(a, k))
	check(out, 0, 0, true, true, true, true)
	set(1, 2, false, false)
	must(m.SetInt32(k, 3))
	must(out.DivScalar(a, k))
	lo, hi, e := out.endpoints()
	must(e)
	lower := new(big.Rat).SetFloat64(m.ToFloat64(lo))
	upper := new(big.Rat).SetFloat64(m.ToFloat64(hi))
	if lower.Cmp(big.NewRat(1, 3)) > 0 || upper.Cmp(big.NewRat(2, 3)) < 0 {
		t.Fatal("directed scalar quotient", lower, upper)
	}
}
func TestSubpavingHardwareIntervalPowerRetainedBranches(t *testing.T) {
	m, a, out, values := hardwareIntervalFixture(t)
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	set := func(l, u int32, lop, uop bool) {
		t.Helper()
		must(m.SetInt32(&values[0], l))
		must(m.SetInt32(&values[1], u))
		must(a.SetFinite(&values[0], &values[1], lop, uop))
	}
	check := func(v *HardwareInterval, l, u float64, li, ui, lop, uop bool) {
		t.Helper()
		lo, hi, e := v.endpoints()
		must(e)
		if v.lowerInf != li || v.upperInf != ui || v.lowerOpen != lop || v.upperOpen != uop || !li && m.ToFloat64(lo) != l || !ui && m.ToFloat64(hi) != u {
			t.Fatalf("power result [%v,%v] inf=(%t,%t) open=(%t,%t)", m.ToFloat64(lo), m.ToFloat64(hi), v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen)
		}
	}
	set(1, 2, false, true)
	must(out.Power(a, 2))
	check(out, 1, 4, false, false, false, true)
	set(-3, -1, true, false)
	must(out.Power(a, 2))
	check(out, 1, 9, false, false, false, true)
	must(a.Power(a, 2))
	check(a, 1, 9, false, false, false, true)
	set(-2, 2, true, false)
	must(out.Power(a, 2))
	check(out, 0, 4, false, false, false, false)
	set(-2, 2, false, true)
	must(out.Power(a, 2))
	check(out, 0, 4, false, false, false, false)
	set(-2, 2, true, true)
	must(out.Power(a, 2))
	check(out, 0, 4, false, false, false, true)
	must(a.ResetLower())
	must(out.Power(a, 2))
	check(out, 0, 0, false, true, false, true)
	set(-3, -2, false, false)
	must(a.ResetLower())
	must(out.Power(a, 2))
	check(out, 4, 0, false, true, false, true)
	set(-2, 3, true, false)
	must(out.Power(a, 3))
	check(out, -8, 27, false, false, true, false)
	must(a.ResetUpper())
	must(out.Power(a, 3))
	check(out, -8, 0, false, true, true, true)
	must(out.Power(a, 0))
	check(out, 0, 0, false, true, false, true)
	// A half-ULP input exercises the directed positive-even power route.
	must(m.SetFloat64(&values[0], math.Nextafter(1, math.Inf(1))))
	must(m.SetFloat64(&values[1], math.Nextafter(1, math.Inf(1))))
	must(a.SetFinite(&values[0], &values[1], false, false))
	must(out.Power(a, 2))
	lo, hi, e := out.endpoints()
	must(e)
	exact := new(big.Rat).SetFloat64(m.ToFloat64(&values[0]))
	exact.Mul(exact, exact)
	if new(big.Rat).SetFloat64(m.ToFloat64(lo)).Cmp(exact) > 0 || new(big.Rat).SetFloat64(m.ToFloat64(hi)).Cmp(exact) < 0 {
		t.Fatal("directed square enclosure")
	}
}
