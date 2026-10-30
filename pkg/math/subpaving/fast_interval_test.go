package subpaving

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type shortOnceSubpavingIntervalWriter struct {
	bytes.Buffer
	at, calls int
}

func (w *shortOnceSubpavingIntervalWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == w.at {
		return w.Buffer.Write(p[:len(p)-1])
	}
	return w.Buffer.Write(p)
}

func (w *shortOnceSubpavingIntervalWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func TestSubpavingFastMutableIntervalRetainedCore(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	cfg := numeral.DefaultFastFloatConfig(arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: b}})
	cfg.InitialCapacity = 4
	m, err := numeral.NewFastFloatManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([]*numeral.ScopedFastFloat, 2)
	for i := range inputs {
		inputs[i], err = numeral.NewScopedFastFloat(m)
		if err != nil {
			t.Fatal(err)
		}
	}
	v, err := NewFastInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	copyOf, err := NewFastInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, x := range []*FastInterval{copyOf, v} {
			if e := x.Close(); e != nil {
				t.Error(e)
			}
		}
		for _, x := range inputs {
			if e := x.Close(); e != nil {
				t.Error(e)
			}
		}
		if e := m.CloseChecked(); e != nil {
			t.Error(e)
		}
		if b.Used() != 0 {
			t.Error("retained fast-float roots", b.Used())
		}
	}()
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	lo, err := inputs[0].Get()
	must(err)
	hi, err := inputs[1].Get()
	must(err)
	var rendered bytes.Buffer
	must(v.Display(&rendered))
	if rendered.String() != "(-oo, +oo)" {
		t.Fatal("initial interval", rendered.String())
	}
	for _, tc := range []struct {
		at   int
		want string
	}{{1, ""}, {3, "(-oo,"}, {5, "(-oo, +oo"}} {
		w := &shortOnceSubpavingIntervalWriter{at: tc.at}
		if err := v.Display(w); !errors.Is(err, io.ErrShortWrite) || w.String() != tc.want || w.calls != tc.at {
			t.Fatalf("short insertion %d = (%q, %d, %v)", tc.at, w.String(), w.calls, err)
		}
	}
	contains, err := v.ContainsZero()
	must(err)
	if !contains {
		t.Fatal("unbounded interval must contain zero")
	}
	must(m.SetInt32(lo, 0))
	must(m.SetInt32(hi, 2))
	must(v.SetFinite(lo, hi, true, false))
	contains, err = v.ContainsZero()
	must(err)
	if contains {
		t.Fatal("open zero lower endpoint contains zero")
	}
	must(v.SetFrom(v))
	must(copyOf.SetFrom(v))
	must(m.SetInt32(lo, -1))
	must(m.SetInt32(hi, -2))
	if err := v.SetFinite(lo, hi, false, false); !errors.Is(err, interval.ErrExtendedDomain) {
		t.Fatal("reversed bounds", err)
	}
	must(m.SetInt32(hi, 2))
	must(v.SetFinite(lo, hi, false, true))
	contains, err = v.ContainsZero()
	must(err)
	if !contains {
		t.Fatal("straddling interval excludes zero")
	}
	must(v.ResetLower())
	must(v.ResetUpper())
	must(v.CheckInvariant())
	rendered.Reset()
	must(v.Display(&rendered))
	if rendered.String() != "(-oo, +oo)" {
		t.Fatal("reset interval", rendered.String())
	}
	must(copyOf.CheckInvariant())
	copyLo, copyHi, err := copyOf.endpoints()
	must(err)
	wantLo, err := m.String(copyLo)
	must(err)
	wantHi, err := m.String(copyHi)
	must(err)
	rendered.Reset()
	must(copyOf.Display(&rendered))
	if rendered.String() != "("+wantLo+", "+wantHi+"]" {
		t.Fatal("copied interval", rendered.String())
	}
	must(v.Close())
	if _, err := v.ContainsZero(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed interval", err)
	}
}

func TestSubpavingFastConstantIntervalSelfAssignment(t *testing.T) {
	m, _, _, root := fastMonomialFixture(t, []bool{false})
	view, err := NewFastInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := view.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := view.SetConstant(root, 0); err != nil {
		t.Fatal(err)
	}
	if err := view.SetFrom(view); err != nil {
		t.Fatalf("constant self-assignment must be a no-op: %v", err)
	}
}
