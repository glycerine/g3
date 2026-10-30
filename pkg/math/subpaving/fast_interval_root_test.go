package subpaving

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFastIntervalXnEqYRetainedCases(t *testing.T) {
	budget := arena.NewBudget(1 << 28)
	cfg := numeral.DefaultFastFloatConfig(arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: budget}})
	cfg.InitialCapacity = 4
	m, err := numeral.NewFastFloatManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	owners := make([]*numeral.ScopedFastFloat, 3)
	for i := range owners {
		owners[i], err = numeral.NewScopedFastFloat(m)
		if err != nil {
			t.Fatal(err)
		}
	}
	values := make([]*numeral.FastFloat, len(owners))
	for i, owner := range owners {
		values[i], err = owner.Get()
		if err != nil {
			t.Fatal(err)
		}
	}
	a, err := NewFastInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewFastInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := b.Close(); e != nil {
			t.Error(e)
		}
		if e := a.Close(); e != nil {
			t.Error(e)
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
			t.Error("retained root owners", budget.Used())
		}
	}()
	lo, hi, p := values[0], values[1], values[2]
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	must(m.SetFraction32(p, 1, 8192))
	limit := &control.Reslimit{Limit: 1000}
	roots := interval.FastRoot{Numbers: m, Limit: limit}
	set := func(l, u int32, lOpen, uOpen bool) {
		t.Helper()
		must(m.SetInt32(lo, l))
		must(m.SetInt32(hi, u))
		must(a.SetFinite(lo, hi, lOpen, uOpen))
	}
	show := func(v *FastInterval) string {
		t.Helper()
		var out bytes.Buffer
		must(v.Display(&out))
		return out.String()
	}
	checkEncloses := func(v *FastInterval, lower, upper int32) {
		t.Helper()
		gotLower, gotUpper, e := v.endpoints()
		must(e)
		must(m.SetInt32(lo, lower))
		must(m.SetInt32(hi, upper))
		tooHigh, e := m.Greater(gotLower, lo)
		must(e)
		tooLow, e := m.Less(gotUpper, hi)
		must(e)
		if tooHigh || tooLow {
			t.Fatalf("inverse misses [%d,%d]: %q", lower, upper, show(v))
		}
	}
	set(4, 9, false, false)
	must(b.XnEqY(a, 2, p, roots))
	checkEncloses(b, -3, 3)
	must(a.XnEqY(a, 2, p, roots)) // even-power output may alias y
	checkEncloses(a, -3, 3)
	set(4, 9, true, true)
	must(b.XnEqY(a, 1, p, roots)) // source n=1 delegates to interval set
	if got := show(b); got != "(4, 9)" {
		t.Fatalf("degree-one inverse = %q", got)
	}
	if e := b.XnEqY(a, 0, p, roots); !errors.Is(e, interval.ErrExtendedDomain) {
		t.Fatal("zero-degree inverse precondition", e)
	}
	set(4, 9, false, true)
	must(b.XnEqY(a, 2, p, roots))
	checkEncloses(b, -3, 3)
	if b.lowerOpen || b.upperOpen {
		t.Fatalf("rounded even inverse retained open endpoints: %q", show(b))
	}
	set(-8, 27, true, false)
	must(b.XnEqY(a, 3, p, roots))
	checkEncloses(b, -2, 3)
	beforeAlias := show(b)
	must(a.XnEqY(a, 3, p, roots))
	if got := show(a); got != beforeAlias {
		t.Fatalf("aliased odd inverse = %q, want %q", got, beforeAlias)
	}
	set(0, 9, false, false)
	must(a.ResetUpper())
	must(b.XnEqY(a, 2, p, roots))
	if got := show(b); got != "(-oo, +oo)" {
		t.Fatalf("unbounded even inverse = %q", got)
	}
	set(-8, 8, false, false)
	must(a.ResetLower())
	must(b.XnEqY(a, 3, p, roots))
	if !b.lowerInf || b.upperInf {
		t.Fatalf("one-sided odd inverse = %q", show(b))
	}
	_, gotUpper, e := b.endpoints()
	must(e)
	must(m.SetInt32(hi, 2))
	tooLow, e := m.Less(gotUpper, hi)
	must(e)
	if tooLow {
		t.Fatalf("one-sided odd upper misses 2: %q", show(b))
	}
	set(0, 2, true, true)
	must(b.XnEqY(a, 2, p, roots))
	if b.lowerOpen || b.upperOpen {
		t.Fatalf("inexact even root retained open endpoints: %q", show(b))
	}
	set(-1, 9, false, false)
	if e := b.XnEqY(a, 2, p, roots); !errors.Is(e, interval.ErrExtendedDomain) {
		t.Fatal("negative even input", e)
	}
	set(2, 4, false, false)
	limit.Limit, limit.Steps = 1, 0
	if e := b.XnEqY(a, 3, p, roots); !errors.Is(e, interval.ErrFastRootCanceled) {
		t.Fatal("root checkpoint", e)
	}
}
