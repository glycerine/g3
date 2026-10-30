package subpaving

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingMPFIntervalPowerRetainedCases(t *testing.T) {
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
	a, err := NewArbitraryInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	out, err := NewArbitraryInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := out.Close(); err != nil {
			t.Error(err)
		}
		if err := a.Close(); err != nil {
			t.Error(err)
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
			t.Error("MPF power roots", budget.Used())
		}
	})
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	set := func(lo, hi int32, lop, uop bool) {
		t.Helper()
		must(m.SetInt32(&values[0], lo))
		must(m.SetInt32(&values[1], hi))
		must(a.SetFinite(&values[0], &values[1], lop, uop))
	}
	check := func(v *ArbitraryInterval, lo, hi int32, li, ui, lop, uop bool) {
		t.Helper()
		actualLo, actualHi, err := v.endpoints()
		must(err)
		if v.lowerInf != li || v.upperInf != ui || v.lowerOpen != lop || v.upperOpen != uop {
			t.Fatalf("power shape inf=(%t,%t) open=(%t,%t)", v.lowerInf, v.upperInf, v.lowerOpen, v.upperOpen)
		}
		for _, c := range []struct {
			actual   *numeral.ArbitraryFloat
			want     int32
			infinite bool
			value    *numeral.ArbitraryFloat
		}{
			{actualLo, lo, li, &values[2]}, {actualHi, hi, ui, &values[3]},
		} {
			if c.infinite {
				continue
			}
			if c.want == 0 {
				zero, err := m.IsZero(c.actual)
				must(err)
				if !zero {
					t.Fatal("power zero endpoint")
				}
				continue
			}
			must(m.SetInt32(c.value, c.want))
			eq, err := m.Equal(c.actual, c.value)
			must(err)
			if !eq {
				t.Fatal("power finite endpoint", c.want)
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
	m.RoundToMinusInf()
	must(m.SetFractionSigned(&values[0], 1, 3))
	m.RoundToPlusInf()
	must(m.SetFractionSigned(&values[1], 2, 3))
	must(a.SetFinite(&values[0], &values[1], false, false))
	must(out.Power(a, 2))
	actualLo, actualHi, err := out.endpoints()
	must(err)
	m.RoundToMinusInf()
	must(m.SetFractionSigned(&values[2], 1, 9))
	m.RoundToPlusInf()
	must(m.SetFractionSigned(&values[3], 4, 9))
	lowerBeyond, err := m.Less(&values[2], actualLo)
	must(err)
	upperShort, err := m.Less(actualHi, &values[3])
	must(err)
	if lowerBeyond || upperShort {
		t.Fatal("directed square excludes rational enclosure")
	}
	must(out.Power(a, 0))
	check(out, 1, 1, false, false, false, false)
}
