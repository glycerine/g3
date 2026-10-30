package subpaving

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFastIntervalMulSignClasses(t *testing.T) {
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
	a, b, out := intervals[0], intervals[1], intervals[2]
	set := func(v *FastInterval, lo, hi int32, loOpen, hiOpen bool) {
		t.Helper()
		must(m.SetInt32(values[0], lo))
		must(m.SetInt32(values[1], hi))
		must(v.SetFinite(values[0], values[1], loOpen, hiOpen))
	}
	check := func(v *FastInterval, lo, hi int32, loOpen, hiOpen bool) {
		t.Helper()
		actualLo, actualHi, e := v.endpoints()
		must(e)
		must(m.SetInt32(values[2], lo))
		must(m.SetInt32(values[3], hi))
		lowerEqual, e := m.Equal(actualLo, values[2])
		must(e)
		upperEqual, e := m.Equal(actualHi, values[3])
		must(e)
		if v.lowerInf || v.upperInf || !lowerEqual || !upperEqual || v.lowerOpen != loOpen || v.upperOpen != hiOpen {
			t.Fatalf("product interval: lower=%t upper=%t open=(%t,%t)", lowerEqual, upperEqual, v.lowerOpen, v.upperOpen)
		}
		must(v.CheckInvariant())
	}
	for _, tc := range []struct {
		name           string
		al, au, bl, bu int32
		lo, hi         int32
	}{
		{"NN", -3, -1, -3, -1, 1, 9},
		{"NM", -3, -1, -2, 2, -6, 6},
		{"NP", -3, -1, 1, 3, -9, -1},
		{"MN", -2, 2, -3, -1, -6, 6},
		{"MM", -2, 2, -2, 2, -4, 4},
		{"MP", -2, 2, 1, 3, -6, 6},
		{"PN", 1, 3, -3, -1, -9, -1},
		{"PM", 1, 3, -2, 2, -6, 6},
		{"PP", 1, 3, 1, 3, 1, 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set(a, tc.al, tc.au, false, false)
			set(b, tc.bl, tc.bu, false, false)
			must(out.Mul(a, b))
			check(out, tc.lo, tc.hi, false, false)
		})
	}
	// When equal mixed-sign extrema have different attainability, the C++
	// manager selects the closed candidate; the Go result must keep that tie.
	set(a, -2, 2, false, false)
	set(b, -2, 2, true, false)
	must(out.Mul(a, b))
	check(out, -4, 4, false, false)
	set(a, 0, 2, true, false) // (0, 2]
	set(b, 0, 3, false, false) // [0, 3]
	must(out.Mul(a, b))
	check(out, 0, 6, false, false)
	set(b, 0, 3, true, false) // (0, 3]
	must(out.Mul(a, b))
	check(out, 0, 6, true, false)
	set(a, -2, 0, false, false)
	set(b, 1, 3, false, false)
	must(out.Mul(a, b))
	check(out, -6, 0, false, false)
	set(a, -2, 0, false, true)
	must(out.Mul(a, b))
	check(out, -6, 0, false, true)
	set(a, -2, 2, true, true)
	set(b, -2, 2, true, true)
	must(a.Mul(a, b))
	check(a, -4, 4, true, true)
	set(a, -2, -1, false, false)
	must(a.ResetLower())
	set(b, 1, 2, false, false)
	must(out.Mul(a, b))
	if !out.lowerInf || out.upperInf {
		t.Fatal("one-sided infinite product", out.lowerInf, out.upperInf)
	}
	_, actualHi, err := out.endpoints()
	must(err)
	must(m.SetInt32(values[2], -1))
	equal, err := m.Equal(actualHi, values[2])
	must(err)
	if !equal || out.upperOpen {
		t.Fatal("one-sided product upper endpoint")
	}
	set(b, 0, 0, false, false)
	must(out.Mul(a, b))
	check(out, 0, 0, false, false)
	must(m.SetFraction32(values[1], 1, 3))
	must(a.SetFinite(values[1], values[1], false, false))
	must(b.SetFinite(values[1], values[1], false, false))
	must(out.Mul(a, b))
	lo, hi, err := out.endpoints()
	must(err)
	less, err := m.Less(lo, hi)
	must(err)
	if !less {
		t.Fatal("fast-float multiplication did not produce directed bounds")
	}
}
