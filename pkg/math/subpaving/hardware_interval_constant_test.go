package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestHardwareIntervalConstantNodeViewAndMutationGuards(t *testing.T) {
	m, err := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewHardwareContext(m, &control.Reslimit{Limit: 1000}, 20, 1, 8192)
	if err != nil {
		t.Fatal(err)
	}
	x, err := c.NewVariable(false)
	if err != nil {
		t.Fatal(err)
	}
	root, err := c.CreateRootNode()
	if err != nil {
		t.Fatal(err)
	}
	view, err := NewHardwareInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewHardwareInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, v := range []*HardwareInterval{view, snapshot} {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := view.SetConstant(root, x); err != nil {
		t.Fatal(err)
	}
	if err := view.SetFrom(view); err != nil {
		t.Fatalf("constant self-assignment must be a no-op: %v", err)
	}
	if !view.lowerInf || !view.upperInf || !view.lowerOpen || !view.upperOpen {
		t.Fatal("unbounded root view")
	}
	var one, two, three numeral.HardwareFloat
	for _, entry := range []struct {
		out   *numeral.HardwareFloat
		value int32
	}{{&one, 1}, {&two, 2}, {&three, 3}} {
		if err := m.SetInt32(entry.out, entry.value); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.PropagateBound(root, x, &one, true, true); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateBound(root, x, &three, false, false); err != nil {
		t.Fatal(err)
	}
	inside, err := view.Contains(&one)
	if err != nil || inside {
		t.Fatal("open lower bound read through live node view", inside, err)
	}
	if err := snapshot.SetFrom(view); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateBound(root, x, &two, true, false); err != nil {
		t.Fatal(err)
	}
	lo, hi, err := view.endpoints()
	if err != nil || m.ToFloat64(lo) != 2 || m.ToFloat64(hi) != 3 || view.lowerOpen || view.upperOpen {
		t.Fatal("constant view must observe later node bounds", err)
	}
	slo, shi, err := snapshot.endpoints()
	if err != nil || m.ToFloat64(slo) != 1 || m.ToFloat64(shi) != 3 || !snapshot.lowerOpen {
		t.Fatal("mutable copy must retain earlier bound snapshot", err)
	}
	for name, mutate := range map[string]func() error{
		"set":   func() error { return view.SetFinite(&one, &three, false, false) },
		"reset": func() error { return view.Reset() },
		"copy":  func() error { return view.SetFrom(snapshot) },
		"add":   func() error { return view.Add(view, snapshot) },
		"mul":   func() error { return view.Mul(view, snapshot) },
		"power": func() error { return view.Power(view, 2) },
	} {
		if err := mutate(); !errors.Is(err, arena.ErrInvalidSlot) {
			t.Fatal(name, "wrote through constant view", err)
		}
	}
	if m.ToFloat64(&root.lower[x].value) != 2 || m.ToFloat64(&root.upper[x].value) != 3 {
		t.Fatal("constant view mutation damaged node bounds")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := view.Contains(&two); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("view of closed node", err)
	}
}
