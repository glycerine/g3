package subpaving

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFastIntervalNegAddSubDirectedAndAlias(t *testing.T) {
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
	for i, value := range []int32{-2, 3, 1, 4} {
		must(m.SetInt32(values[i], value))
	}
	a, b, out := intervals[0], intervals[1], intervals[2]
	must(a.SetFinite(values[0], values[1], false, true))
	must(b.SetFinite(values[2], values[3], true, false))
	check := func(v *FastInterval, lower, upper int32, lowerOpen, upperOpen bool) {
		t.Helper()
		lo, hi, e := v.endpoints()
		must(e)
		must(m.SetInt32(values[0], lower))
		must(m.SetInt32(values[1], upper))
		le, e := m.Equal(lo, values[0])
		must(e)
		ue, e := m.Equal(hi, values[1])
		must(e)
		if v.lowerInf || v.upperInf || !le || !ue || v.lowerOpen != lowerOpen || v.upperOpen != upperOpen {
			t.Fatalf("interval endpoints: lower=%t upper=%t open=(%t,%t)", le, ue, v.lowerOpen, v.upperOpen)
		}
	}
	must(out.Add(a, b))
	check(out, -1, 7, true, true)
	must(out.Sub(a, b))
	check(out, -6, 2, false, true)
	must(out.Neg(a))
	check(out, -3, 2, true, false)
	must(a.Add(a, b))
	check(a, -1, 7, true, true)
	must(b.Sub(b, b))
	check(b, -3, 3, true, true)
	must(a.Neg(a))
	check(a, -7, 1, true, true)
	must(out.Reset())
	must(out.Add(out, b))
	if !out.lowerInf || !out.upperInf {
		t.Fatal("unbounded plus finite must stay unbounded")
	}
	must(out.Neg(out))
	if !out.lowerInf || !out.upperInf {
		t.Fatal("negated unbounded interval must stay unbounded")
	}
	must(m.SetInt32(values[0], 1))
	must(m.SetFraction32(values[2], 1, 3))
	must(a.SetFinite(values[0], values[0], false, false))
	must(b.SetFinite(values[2], values[2], false, false))
	must(out.Add(a, b))
	roundedLower, roundedUpper, err := out.endpoints()
	must(err)
	less, err := m.Less(roundedLower, roundedUpper)
	must(err)
	if !less {
		t.Fatal("fast-float addition did not apply opposite rounding directions")
	}
	// C++ negation swaps one-sided infinite bounds and their openness. Subtraction
	// combines the upper bound of the second interval with the first's lower.
	must(m.SetInt32(values[0], -2))
	must(m.SetInt32(values[1], 3))
	must(a.SetFinite(values[0], values[1], false, false))
	must(a.ResetLower()) // (-oo, 3]
	must(out.Neg(a))
	var rendered bytes.Buffer
	must(out.Display(&rendered))
	if rendered.String() != "[-3, +oo)" {
		t.Fatal("negated upper-bounded interval", rendered.String())
	}
	must(b.SetFinite(values[0], values[1], false, false))
	must(b.ResetUpper()) // [-2, +oo)
	must(out.Neg(b))
	rendered.Reset()
	must(out.Display(&rendered))
	if rendered.String() != "(-oo, 2]" {
		t.Fatal("negated lower-bounded interval", rendered.String())
	}
	must(out.Sub(a, b))
	rendered.Reset()
	must(out.Display(&rendered))
	if rendered.String() != "(-oo, 5]" {
		t.Fatal("one-sided subtraction", rendered.String())
	}
	for _, v := range intervals {
		must(v.CheckInvariant())
	}
}
