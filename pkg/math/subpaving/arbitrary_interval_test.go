package subpaving

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingMPFMutableIntervalRetainedCore(t *testing.T) {
	b := arena.NewBudget(1 << 27)
	r, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	if err != nil {
		t.Fatal(err)
	}
	core, err := numeral.NewArbitraryFloatManager(numeral.IntegerConfig{
		Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}},
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := numeral.NewArbitraryFloatNumeralManager(core, 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewArbitraryInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	copyOf, err := NewArbitraryInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	var lo, hi, nan numeral.ArbitraryFloat
	t.Cleanup(func() {
		for _, x := range []*ArbitraryInterval{copyOf, v} {
			if err := x.Close(); err != nil {
				t.Error(err)
			}
		}
		for _, x := range []*numeral.ArbitraryFloat{&lo, &hi, &nan} {
			if err := m.Del(x); err != nil {
				t.Error(err)
			}
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
		if err := core.Close(); err != nil {
			t.Error(err)
		}
		if err := r.Close(); err != nil {
			t.Error(err)
		}
		if b.Used() != 0 {
			t.Error("MPF interval owners", b.Used())
		}
	})
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var rendered bytes.Buffer
	must(v.Display(&rendered))
	if rendered.String() != "(-oo, +oo)" {
		t.Fatal("unbounded display", rendered.String())
	}
	contains, err := v.ContainsZero()
	must(err)
	if !contains {
		t.Fatal("unbounded contains zero")
	}
	must(m.SetInt32(&lo, -1))
	must(m.SetInt32(&hi, 2))
	must(v.SetFinite(&lo, &hi, false, true))
	must(v.SetFrom(v))
	must(core.MkNaN(11, 53, &nan, nil))
	for _, bounds := range [][2]*numeral.ArbitraryFloat{{&nan, &nan}, {&nan, &hi}, {&lo, &nan}} {
		if err := v.SetFinite(bounds[0], bounds[1], false, false); !errors.Is(err, interval.ErrExtendedDomain) {
			t.Fatalf("NaN bounds accepted: %v", err)
		}
		must(v.CheckInvariant())
	}
	must(copyOf.SetFrom(v))
	must(copyOf.CheckInvariant())
	rendered.Reset()
	must(copyOf.Display(&rendered))
	if rendered.String() != "[-1.0 0, 1.0 1)" {
		t.Fatal("finite display", rendered.String())
	}
	contains, err = v.ContainsZero()
	must(err)
	if !contains {
		t.Fatal("straddling interval excludes zero")
	}
	if err := v.SetFinite(&hi, &lo, false, false); !errors.Is(err, interval.ErrExtendedDomain) {
		t.Fatal("reversed bounds", err)
	}
	if err := v.SetFinite(&lo, &lo, true, false); !errors.Is(err, interval.ErrExtendedDomain) {
		t.Fatal("open singleton", err)
	}
	must(v.SetFinite(&lo, &lo, false, false))
	must(v.CheckInvariant())
	must(v.ResetLower())
	must(v.ResetUpper())
	rendered.Reset()
	must(v.Display(&rendered))
	if rendered.String() != "(-oo, +oo)" {
		t.Fatal("reset display", rendered.String())
	}
	w := &shortOnceSubpavingIntervalWriter{at: 3}
	if err := v.Display(w); !errors.Is(err, io.ErrShortWrite) || w.String() != "(-oo," {
		t.Fatal("short writer", w.String(), err)
	}
	must(v.Close())
	if _, err := v.ContainsZero(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed interval", err)
	}
}
