package subpaving

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSubpavingHardwareContextReadsParamsRefInSourceOrder(t *testing.T) {
	b := arena.NewBudget(1 << 26)
	sy, err := symbol.NewManager(symbol.Config{Budget: b, PageSize: 64, DescriptorPageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{PageSize: 2, Budget: b}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: b}}}})
	if err != nil {
		t.Fatal(err)
	}
	pc, err := uparams.NewParamsContext(sy, q, uparams.ParamsConfig{Budget: b, PageEntries: 1, Text: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: b}}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := pc.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pc.Del(&p); err != nil {
			t.Error(err)
		}
		if err := pc.Close(); err != nil {
			t.Error(err)
		}
		if err := q.Close(); err != nil {
			t.Error(err)
		}
		if err := sy.Close(); err != nil {
			t.Error(err)
		}
		if b.Used() != 0 {
			t.Error("parameter owner roots", b.Used())
		}
	})
	m, err := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	qm, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{PageSize: 2, Budget: b}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: b}}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := qm.CloseChecked(); err != nil {
			t.Error(err)
		}
	})
	c, err := NewHardwareContext(m, &control.Reslimit{Limit: 1000}, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewHardwareShellCoefficients(c, qm)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	set := func(name string, value uint32) {
		t.Helper()
		if err := pc.SetUIntName(&p, &name, value); err != nil {
			t.Fatal(err)
		}
	}
	set("epsilon", 4)
	set("max_bound", 2)
	set("max_depth", 7)
	set("max_nodes", 9)
	set("max_memory", 3)
	set("nth_root_precision", 16)
	if err := w.UpdateParamsRef(pc, &p); err != nil {
		t.Fatal(err)
	}
	if c.zeroEpsilon || m.ToFloat64(&c.epsilon) != .25 || m.ToFloat64(&c.maxBound) != 100 || m.ToFloat64(&c.minusMaxBound) != -100 || m.ToFloat64(&c.rootPrecision) != 1.0/16 || c.maxDepth != 7 || c.maxNodes != 9 || c.maxMemory != 3*1024*1024 {
		t.Fatal("named hardware context parameters")
	}
	constructed, err := NewHardwareContextFromParams(m, &control.Reslimit{Limit: 1000}, pc, &p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := constructed.Close(); err != nil {
			t.Error(err)
		}
	})
	if constructed.root != nil || constructed.zeroEpsilon || m.ToFloat64(&constructed.epsilon) != .25 ||
		m.ToFloat64(&constructed.maxBound) != 100 || m.ToFloat64(&constructed.minusMaxBound) != -100 ||
		m.ToFloat64(&constructed.rootPrecision) != 1.0/16 || constructed.maxDepth != 7 ||
		constructed.maxNodes != 9 || constructed.maxMemory != 3*1024*1024 {
		t.Fatal("constructor must apply the source parameter pass before root initialization")
	}
	set("epsilon", 2)
	set("max_bound", 1<<31)
	if err := w.UpdateParamsRef(pc, &p); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("late power guard", err)
	}
	if m.ToFloat64(&c.epsilon) != .5 || m.ToFloat64(&c.maxBound) != 10 || m.ToFloat64(&c.minusMaxBound) != -100 || c.maxDepth != 7 || c.maxNodes != 9 {
		t.Fatal("source getter/mutation prefix")
	}
	if err := pc.Reset(&p); err != nil {
		t.Fatal(err)
	}
	if err := w.UpdateParamsRef(pc, &p); err != nil {
		t.Fatal(err)
	}
	if c.zeroEpsilon || m.ToFloat64(&c.maxBound) != 1e10 || m.ToFloat64(&c.rootPrecision) != 1.0/8192 || c.maxDepth != 128 || c.maxNodes != 8192 || c.maxMemory != math.MaxUint64 {
		t.Fatal("source named defaults")
	}
	if err := w.UpdateParamsRef(nil, &p); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("missing parameter context", err)
	}
}
