package subpaving

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingHardwareIntervalDivSignBranchesAndDirectedBounds(t *testing.T) {
	m, err := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	var intervals [3]*HardwareInterval
	for i := range intervals {
		intervals[i], err = NewHardwareInterval(m)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for i := len(intervals) - 1; i >= 0; i-- {
			if err := intervals[i].Close(); err != nil {
				t.Error(err)
			}
		}
	})
	num, den, out := intervals[0], intervals[1], intervals[2]
	var values [2]numeral.HardwareFloat
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	set := func(v *HardwareInterval, lo, hi int32, loOpen, hiOpen bool) {
		t.Helper()
		must(m.SetInt32(&values[0], lo))
		must(m.SetInt32(&values[1], hi))
		must(v.SetFinite(&values[0], &values[1], loOpen, hiOpen))
	}
	check := func(v *HardwareInterval, lo, hi float64, lowerInf, upperInf, lowerOpen, upperOpen bool) {
		t.Helper()
		a, b, err := v.endpoints()
		must(err)
		if v.lowerInf != lowerInf || v.upperInf != upperInf || v.lowerOpen != lowerOpen || v.upperOpen != upperOpen ||
			(!lowerInf && m.ToFloat64(a) != lo) || (!upperInf && m.ToFloat64(b) != hi) {
			t.Fatalf("quotient [%g,%g] inf=(%t,%t) open=(%t,%t)", m.ToFloat64(a), m.ToFloat64(b), v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen)
		}
		must(v.CheckInvariant())
	}
	for _, tc := range []struct {
		name                   string
		al, au, bl, bu, lo, hi int32
	}{
		{"NN", -4, -2, -2, -1, 1, 4}, {"NP", -4, -2, 1, 2, -4, -1},
		{"MN", -2, 2, -2, -1, -2, 2}, {"MP", -2, 2, 1, 2, -2, 2},
		{"PN", 2, 4, -2, -1, -4, -1}, {"PP", 2, 4, 1, 2, 1, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set(num, tc.al, tc.au, false, false)
			set(den, tc.bl, tc.bu, false, false)
			must(out.Div(num, den))
			check(out, float64(tc.lo), float64(tc.hi), false, false, false, false)
		})
	}
	set(num, 1, 2, false, false)
	set(den, 0, 2, true, false)
	must(out.Div(num, den))
	check(out, .5, 0, false, true, false, true)
	set(num, -2, -1, false, false)
	must(out.Div(num, den))
	check(out, 0, -.5, true, false, true, false)
	set(num, -1, 1, false, false)
	must(out.Div(num, den))
	check(out, 0, 0, true, true, true, true)
	set(num, 1, 2, false, false)
	set(den, -2, 0, false, true)
	must(out.Div(num, den))
	check(out, 0, -.5, true, false, true, false)
	set(num, 0, 0, false, false)
	must(out.Div(num, den))
	check(out, 0, 0, false, false, false, false)
	// math_subpaving_01-F1: the source only SASSERTs a zero-free denominator.
	// A closed zero lower endpoint takes the P1 branches, where the raw
	// m().is_zero(c) test turns the c-quotient into an infinity.
	set(num, 1, 8, false, false)
	set(den, 0, 4, false, false)
	must(out.Div(num, den))
	check(out, .25, 0, false, true, false, false)
	set(num, -8, -1, false, false)
	must(out.Div(num, den))
	check(out, 0, -.25, true, false, false, false)
	// A straddling denominator with nonzero endpoints yields an inverted
	// result in the source; the Go invariant check reports it.
	set(num, 1, 2, false, false)
	set(den, -1, 1, false, false)
	if err := out.Div(num, den); !errors.Is(err, interval.ErrExtendedDomain) {
		t.Fatal("zero-containing denominator", err)
	}
	if err := num.Div(num, den); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("numerator alias", err)
	}
	set(num, 2, 4, false, false)
	set(den, 1, 2, false, false)
	must(den.Div(num, den))
	check(den, 1, 4, false, false, false, false)
	set(num, 1, 1, false, false)
	set(den, 3, 3, false, false)
	must(out.Div(num, den))
	lo, hi, err := out.endpoints()
	must(err)
	l := new(big.Rat).SetFloat64(m.ToFloat64(lo))
	u := new(big.Rat).SetFloat64(m.ToFloat64(hi))
	if l.Cmp(big.NewRat(1, 3)) >= 0 || u.Cmp(big.NewRat(1, 3)) <= 0 {
		t.Fatal("directed binary64 quotient", l, u)
	}
}
