package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingMPFIntervalDivSixSignBranchesAndOpenZero(t *testing.T) {
	budget := arena.NewBudget(1 << 27)
	runtime, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, SizeTBits: 64})
	if err != nil {
		t.Fatal(err)
	}
	core, err := numeral.NewArbitraryFloatManager(numeral.IntegerConfig{
		Memory: runtime, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := numeral.NewArbitraryFloatNumeralManager(core, 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	var values [4]numeral.ArbitraryFloat
	var intervals [3]*ArbitraryInterval
	for i := range intervals {
		intervals[i], err = NewArbitraryInterval(m)
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
		for i := len(values) - 1; i >= 0; i-- {
			if err := m.Del(&values[i]); err != nil {
				t.Error(err)
			}
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
		if err := core.Close(); err != nil {
			t.Error(err)
		}
		if err := runtime.Close(); err != nil {
			t.Error(err)
		}
		if budget.Used() != 0 {
			t.Error("MPF quotient roots", budget.Used())
		}
	})
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	num, den, out := intervals[0], intervals[1], intervals[2]
	set := func(v *ArbitraryInterval, lo, hi int32) {
		t.Helper()
		must(m.SetInt32(&values[0], lo))
		must(m.SetInt32(&values[1], hi))
		must(v.SetFinite(&values[0], &values[1], false, false))
	}
	shape := func(v *ArbitraryInterval, li, ui, lo, uo bool) {
		t.Helper()
		if v.lowerInf != li || v.upperInf != ui || v.lowerOpen != lo || v.upperOpen != uo {
			t.Fatalf("quotient shape inf=(%t,%t) open=(%t,%t)", v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen)
		}
		must(v.CheckInvariant())
	}
	check := func(v *ArbitraryInterval, lo, hi int32, li, ui, lop, uop bool) {
		t.Helper()
		actualLo, actualHi, err := v.endpoints()
		must(err)
		shape(v, li, ui, lop, uop)
		if !li {
			must(m.SetInt32(&values[2], lo))
			eq, err := m.Equal(actualLo, &values[2])
			must(err)
			if !eq {
				t.Fatal("lower quotient")
			}
		}
		if !ui {
			must(m.SetInt32(&values[3], hi))
			eq, err := m.Equal(actualHi, &values[3])
			must(err)
			if !eq {
				t.Fatal("upper quotient")
			}
		}
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
			set(num, tc.al, tc.au)
			set(den, tc.bl, tc.bu)
			must(out.Div(num, den))
			check(out, tc.lo, tc.hi, false, false, false, false)
		})
	}
	set(num, 1, 2)
	// An open zero endpoint is formed by the source reset route; f2n's
	// checked copy setter refuses a formatted zero.
	must(m.Reset(&den.lower))
	must(m.SetInt32(&den.upper, 2))
	den.lowerInf, den.upperInf, den.lowerOpen, den.upperOpen = false, false, true, false
	must(den.CheckInvariant())
	must(out.Div(num, den))
	shape(out, false, true, false, true)
	lo, _, err := out.endpoints()
	must(err)
	must(m.SetFractionSigned(&values[2], 1, 2))
	eq, err := m.Equal(lo, &values[2])
	must(err)
	if !eq {
		t.Fatal("positive open-zero lower quotient")
	}
	set(num, -2, -1)
	must(out.Div(num, den))
	shape(out, true, false, true, false)
	_, hi, err := out.endpoints()
	must(err)
	must(m.SetFractionSigned(&values[2], -1, 2))
	eq, err = m.Equal(hi, &values[2])
	must(err)
	if !eq {
		t.Fatal("negative open-zero upper quotient")
	}
	must(m.SetInt32(&den.lower, -2))
	must(m.Reset(&den.upper))
	den.lowerOpen, den.upperOpen = false, true
	set(num, 1, 2)
	must(den.CheckInvariant())
	must(out.Div(num, den))
	shape(out, true, false, true, false)
	set(num, -2, -1)
	must(out.Div(num, den))
	shape(out, false, true, false, true)
	// Reset creates a finite zero numerator in the source's early return.
	must(m.Reset(&num.lower))
	must(m.Reset(&num.upper))
	num.lowerInf, num.upperInf, num.lowerOpen, num.upperOpen = false, false, false, false
	must(num.CheckInvariant())
	must(out.Div(num, den))
	shape(out, false, false, false, false)
	zero, err := m.IsZero(&out.lower)
	must(err)
	if !zero {
		t.Fatal("zero numerator quotient")
	}
	// math_subpaving_01-F1: the source only SASSERTs a zero-free denominator.
	// A closed zero lower endpoint takes the P1 branches, where the raw
	// m().is_zero(c) test turns the c-quotient into an infinity.
	set(num, 1, 8)
	must(m.Reset(&den.lower))
	must(m.SetInt32(&den.upper, 4))
	den.lowerInf, den.upperInf, den.lowerOpen, den.upperOpen = false, false, false, false
	must(out.Div(num, den))
	shape(out, false, true, false, false)
	lo, _, err = out.endpoints()
	must(err)
	must(m.SetFractionSigned(&values[2], 1, 4))
	eq, err = m.Equal(lo, &values[2])
	must(err)
	if !eq {
		t.Fatal("positive over closed-zero denominator lower quotient")
	}
	set(num, -8, -1)
	must(out.Div(num, den))
	shape(out, true, false, false, false)
	_, hi, err = out.endpoints()
	must(err)
	must(m.SetFractionSigned(&values[2], -1, 4))
	eq, err = m.Equal(hi, &values[2])
	must(err)
	if !eq {
		t.Fatal("negative over closed-zero denominator upper quotient")
	}
	// A straddling denominator with nonzero endpoints yields an inverted
	// result in the source; the Go invariant check reports it.
	set(num, 1, 2)
	set(den, -1, 1)
	if err := out.Div(num, den); !errors.Is(err, interval.ErrExtendedDomain) {
		t.Fatal("zero-containing denominator", err)
	}
	if err := num.Div(num, den); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("numerator alias", err)
	}
	set(num, 2, 4)
	set(den, 1, 2)
	must(den.Div(num, den))
	check(den, 1, 4, false, false, false, false)
	set(num, 1, 2)
	set(den, 1, 2)
	must(den.ResetUpper())
	must(out.Div(num, den))
	shape(out, false, false, true, false)
	set(num, 1, 1)
	set(den, 3, 3)
	must(out.Div(num, den))
	lo, hi, err = out.endpoints()
	must(err)
	less, err := m.Less(lo, hi)
	must(err)
	if !less {
		t.Fatal("directed MPF quotient bounds collapsed")
	}
}
