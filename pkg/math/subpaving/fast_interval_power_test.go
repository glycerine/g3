package subpaving

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFastIntervalPowerRetainedCases(t *testing.T) {
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
	intervals := make([]*FastInterval, 2)
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
	a, out := intervals[0], intervals[1]
	set := func(lo, hi int32, lowerOpen, upperOpen bool) {
		t.Helper()
		must(m.SetInt32(values[0], lo))
		must(m.SetInt32(values[1], hi))
		must(a.SetFinite(values[0], values[1], lowerOpen, upperOpen))
	}
	check := func(v *FastInterval, lo, hi int32, lowerInf, upperInf, lowerOpen, upperOpen bool) {
		t.Helper()
		actualLo, actualHi, e := v.endpoints()
		must(e)
		if v.lowerInf != lowerInf || v.upperInf != upperInf || v.lowerOpen != lowerOpen || v.upperOpen != upperOpen {
			t.Fatalf("power shape: inf=(%t,%t) open=(%t,%t)", v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen)
		}
		if !lowerInf {
			must(m.SetInt32(values[2], lo))
			equal, e := m.Equal(actualLo, values[2])
			must(e)
			if !equal {
				t.Fatal("power lower endpoint")
			}
		}
		if !upperInf {
			must(m.SetInt32(values[3], hi))
			equal, e := m.Equal(actualHi, values[3])
			must(e)
			if !equal {
				t.Fatal("power upper endpoint")
			}
		}
		must(v.CheckInvariant())
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
	must(m.SetRounding(false))
	must(m.SetFraction32(values[0], 1, 3))
	must(m.SetRounding(true))
	must(m.SetFraction32(values[1], 2, 3))
	must(a.SetFinite(values[0], values[1], false, false))
	must(out.Power(a, 2))
	actualLo, actualHi, err := out.endpoints()
	must(err)
	must(m.SetRounding(false))
	must(m.SetFraction32(values[2], 1, 9))
	must(m.SetRounding(true))
	must(m.SetFraction32(values[3], 4, 9))
	lowerBeyond, err := m.Less(values[2], actualLo)
	must(err)
	upperShort, err := m.Less(actualHi, values[3])
	must(err)
	if lowerBeyond || upperShort {
		t.Fatal("directed squared bounds exclude rational enclosure")
	}
	must(out.Power(a, 0))
	check(out, 1, 1, false, false, false, false)
}
