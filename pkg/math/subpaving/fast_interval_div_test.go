package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFastIntervalDivSignBranches(t *testing.T) {
	budget := arena.NewBudget(1 << 28)
	cfg := numeral.DefaultFastFloatConfig(arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: budget}})
	cfg.InitialCapacity = 4
	m, err := numeral.NewFastFloatManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	owners := make([]*numeral.ScopedFastFloat, 4)
	for i := range owners {
		owners[i], err = numeral.NewScopedFastFloat(m)
		if err != nil {
			t.Fatal(err)
		}
	}
	intervals := make([]*FastInterval, 3)
	for i := range intervals {
		intervals[i], err = NewFastInterval(m)
		if err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		for i := len(intervals) - 1; i >= 0; i-- {
			if e := intervals[i].Close(); e != nil {
				t.Error(e)
			}
		}
		for i := len(owners) - 1; i >= 0; i-- {
			if e := owners[i].Close(); e != nil {
				t.Error(e)
			}
		}
		if e := m.CloseChecked(); e != nil {
			t.Error(e)
		}
		if budget.Used() != 0 {
			t.Error("retained fast-float roots", budget.Used())
		}
	}()
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	values := make([]*numeral.FastFloat, len(owners))
	for i, owner := range owners {
		values[i], err = owner.Get()
		must(err)
	}
	num, den, out := intervals[0], intervals[1], intervals[2]
	set := func(v *FastInterval, lo, hi int32, loOpen, hiOpen bool) {
		t.Helper()
		must(m.SetInt32(values[0], lo))
		must(m.SetInt32(values[1], hi))
		must(v.SetFinite(values[0], values[1], loOpen, hiOpen))
	}
	shape := func(v *FastInterval, lowerInf, upperInf, lowerOpen, upperOpen bool) {
		t.Helper()
		if v.lowerInf != lowerInf || v.upperInf != upperInf || v.lowerOpen != lowerOpen || v.upperOpen != upperOpen {
			t.Fatalf("quotient shape: inf=(%t,%t) open=(%t,%t)", v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen)
		}
		must(v.CheckInvariant())
	}
	check := func(v *FastInterval, lo, hi int32, lowerInf, upperInf, lowerOpen, upperOpen bool) {
		t.Helper()
		actualLo, actualHi, e := v.endpoints()
		must(e)
		shape(v, lowerInf, upperInf, lowerOpen, upperOpen)
		if !lowerInf {
			must(m.SetInt32(values[2], lo))
			equal, e := m.Equal(actualLo, values[2])
			must(e)
			if !equal {
				t.Fatal("lower quotient")
			}
		}
		if !upperInf {
			must(m.SetInt32(values[3], hi))
			equal, e := m.Equal(actualHi, values[3])
			must(e)
			if !equal {
				t.Fatal("upper quotient")
			}
		}
	}
	for _, tc := range []struct {
		name           string
		al, au, bl, bu int32
		lo, hi         int32
	}{
		{"NN", -4, -2, -2, -1, 1, 4},
		{"NP", -4, -2, 1, 2, -4, -1},
		{"MN", -2, 2, -2, -1, -2, 2},
		{"MP", -2, 2, 1, 2, -2, 2},
		{"PN", 2, 4, -2, -1, -4, -1},
		{"PP", 2, 4, 1, 2, 1, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set(num, tc.al, tc.au, false, false)
			set(den, tc.bl, tc.bu, false, false)
			must(out.Div(num, den))
			check(out, tc.lo, tc.hi, false, false, false, false)
		})
	}
	set(num, 1, 2, false, false)
	set(den, 0, 2, true, false)
	must(out.Div(num, den))
	shape(out, false, true, false, true)
	lower, _, err := out.endpoints()
	must(err)
	must(m.SetFraction32(values[2], 1, 2))
	equal, err := m.Equal(lower, values[2])
	must(err)
	if !equal {
		t.Fatal("positive open-zero lower quotient")
	}
	set(num, -2, -1, false, false)
	must(out.Div(num, den))
	shape(out, true, false, true, false)
	_, upper, err := out.endpoints()
	must(err)
	must(m.SetFraction32(values[2], -1, 2))
	equal, err = m.Equal(upper, values[2])
	must(err)
	if !equal {
		t.Fatal("negative open-zero upper quotient")
	}
	set(num, -1, 1, false, false)
	must(out.Div(num, den))
	check(out, 0, 0, true, true, true, true)
	set(den, -2, 0, false, true)
	set(num, 1, 2, false, false)
	must(out.Div(num, den))
	shape(out, true, false, true, false)
	_, upper, err = out.endpoints()
	must(err)
	must(m.SetFraction32(values[2], -1, 2))
	equal, err = m.Equal(upper, values[2])
	must(err)
	if !equal {
		t.Fatal("positive over negative open-zero upper quotient")
	}
	set(num, -2, -1, false, false)
	must(out.Div(num, den))
	shape(out, false, true, false, true)
	lower, _, err = out.endpoints()
	must(err)
	must(m.SetFraction32(values[2], 1, 2))
	equal, err = m.Equal(lower, values[2])
	must(err)
	if !equal {
		t.Fatal("negative over negative open-zero lower quotient")
	}
	set(num, -1, 1, false, false)
	must(out.Div(num, den))
	check(out, 0, 0, true, true, true, true)
	set(num, 0, 0, false, false)
	must(out.Div(num, den))
	check(out, 0, 0, false, false, false, false)
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
	set(num, 1, 2, false, false)
	set(den, 1, 2, false, false)
	must(den.ResetUpper())
	must(out.Div(num, den))
	check(out, 0, 2, false, false, true, false)
	// Exercise infinite endpoints on the numerator and denominator separately.
	set(num, -4, -2, false, false)
	must(num.ResetLower()) // (-oo, -2]
	set(den, 1, 2, false, false)
	must(out.Div(num, den))
	shape(out, true, false, true, false)
	_, upper, err = out.endpoints()
	must(err)
	must(m.SetInt32(values[2], -1))
	equal, err = m.Equal(upper, values[2])
	must(err)
	if !equal {
		t.Fatal("negative infinite numerator quotient")
	}
	set(num, 2, 4, false, false)
	set(den, -2, -1, false, false)
	must(den.ResetLower()) // (-oo, -1]
	must(out.Div(num, den))
	shape(out, false, false, false, true)
	lower, upper, err = out.endpoints()
	must(err)
	must(m.SetInt32(values[2], -4))
	equal, err = m.Equal(lower, values[2])
	must(err)
	must(m.SetInt32(values[2], 0))
	upperEqual, err := m.Equal(upper, values[2])
	must(err)
	if !equal || !upperEqual {
		t.Fatal("positive numerator over negative infinite denominator")
	}
	set(num, 1, 1, false, false)
	set(den, 3, 3, false, false)
	must(out.Div(num, den))
	lo, hi, err := out.endpoints()
	must(err)
	less, err := m.Less(lo, hi)
	must(err)
	if !less {
		t.Fatal("fast-float division did not produce directed bounds")
	}
}
