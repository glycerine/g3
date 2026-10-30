package subpaving

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFastIntervalScalarMulDiv(t *testing.T) {
	budget := arena.NewBudget(1 << 28)
	cfg := numeral.DefaultFastFloatConfig(arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: budget}})
	cfg.InitialCapacity = 4
	m, err := numeral.NewFastFloatManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	owners := make([]*numeral.ScopedFastFloat, 5)
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
	k := values[4]
	check := func(v *FastInterval, lo, hi int32, lowerInf, upperInf, lowerOpen, upperOpen bool) {
		t.Helper()
		actualLo, actualHi, e := v.endpoints()
		must(e)
		if v.lowerInf != lowerInf || v.upperInf != upperInf || v.lowerOpen != lowerOpen || v.upperOpen != upperOpen {
			t.Fatalf("scalar interval shape: inf=(%t,%t) open=(%t,%t)", v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen)
		}
		if !lowerInf {
			must(m.SetInt32(values[2], lo))
			equal, e := m.Equal(actualLo, values[2])
			must(e)
			if !equal {
				t.Fatal("scalar lower endpoint")
			}
		}
		if !upperInf {
			must(m.SetInt32(values[3], hi))
			equal, e := m.Equal(actualHi, values[3])
			must(e)
			if !equal {
				t.Fatal("scalar upper endpoint")
			}
		}
		must(v.CheckInvariant())
	}
	must(m.SetInt32(values[0], -1))
	must(m.SetInt32(values[1], 2))
	must(a.SetFinite(values[0], values[1], true, false))
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
	must(m.SetInt32(k, 0))
	must(out.MulScalar(k, a))
	check(out, 0, 0, true, true, true, true)
	must(out.DivScalar(a, k))
	check(out, 0, 0, true, true, true, true)
	must(m.SetInt32(values[0], 1))
	must(m.SetInt32(values[1], 2))
	must(a.SetFinite(values[0], values[1], false, false))
	must(m.SetInt32(k, 3))
	must(out.MulScalar(k, a))
	check(out, 3, 6, false, false, false, false)
	must(out.DivScalar(a, k))
	lo, hi, err := out.endpoints()
	must(err)
	must(m.SetRounding(false))
	must(m.SetFraction32(values[0], 1, 3))
	must(m.SetRounding(true))
	must(m.SetFraction32(values[1], 2, 3))
	lowerBeyond, err := m.Less(values[0], lo)
	must(err)
	upperShort, err := m.Less(hi, values[1])
	must(err)
	if lowerBeyond || upperShort {
		t.Fatal("directed scalar quotient excludes exact rational endpoints")
	}
	must(out.CheckInvariant())
}
