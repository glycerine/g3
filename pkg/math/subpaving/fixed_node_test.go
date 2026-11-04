package subpaving

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFixedNodeConstantIntervalViews(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := numeral.NewFixedPointManager(2, 1)
	must(err)
	value, err := m.New()
	must(err)
	scratchLower, err := m.New()
	must(err)
	scratchUpper, err := m.New()
	must(err)
	root, err := NewFixedNode(m, 2, nil)
	must(err)
	view, err := NewFixedInterval(m)
	must(err)
	other, err := NewFixedInterval(m)
	must(err)
	result, err := NewFixedInterval(m)
	must(err)
	must(m.SetInt64(scratchLower, -5))
	must(m.SetInt64(scratchUpper, 6))
	must(view.SetFinite(scratchLower, scratchUpper, true, false))
	show := func(v *FixedInterval) string {
		t.Helper()
		var out bytes.Buffer
		must(v.Display(&out))
		return out.String()
	}
	push := func(n *FixedNode, x uint32, k int64, lower, open bool) {
		t.Helper()
		must(m.SetInt64(value, k))
		must(n.PushBound(x, value, lower, open))
	}
	must(view.SetConstant(root, 0))
	must(view.SetFrom(view))
	if got := show(view); got != "(-oo, +oo)" {
		t.Fatalf("unbounded view = %q", got)
	}
	push(root, 0, -2, true, true)
	if got := show(view); got != "(-2, +oo)" {
		t.Fatalf("live lower view = %q", got)
	}
	push(root, 0, 3, false, false)
	if got := show(view); got != "(-2, 3]" {
		t.Fatalf("live two-sided view = %q", got)
	}
	must(result.Power(view, 2))
	if got := show(result); got != "[0, 9]" {
		t.Fatalf("view power = %q", got)
	}
	must(m.SetInt64(value, 2))
	must(other.SetFinite(value, value, false, false))
	must(result.Mul(view, other))
	if got := show(result); got != "(-4, 6]" {
		t.Fatalf("view product = %q", got)
	}
	child, err := NewFixedNode(m, 2, root)
	must(err)
	childView, err := NewFixedInterval(m)
	must(err)
	must(childView.SetConstant(child, 0))
	push(root, 0, 1, true, false)
	push(root, 0, 4, false, false)
	if got := show(view); got != "[1, 4]" {
		t.Fatalf("updated root view = %q", got)
	}
	if got := show(childView); got != "(-2, 3]" {
		t.Fatalf("persistent child view = %q", got)
	}
	push(child, 0, 2, false, true)
	if got := show(childView); got != "(-2, 2)" {
		t.Fatalf("updated child view = %q", got)
	}
	must(result.Div(view, other))
	if got := show(result); got != "[1/2, 2]" {
		t.Fatalf("root view quotient = %q", got)
	}
	if err := view.SetFinite(value, value, false, false); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("constant view mutation error = %v", err)
	}
	if err := view.Reset(); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("constant view reset error = %v", err)
	}
	if err := view.Mul(other, other); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("constant view output error = %v", err)
	}
	if err := root.Close(); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("parent close with child error = %v", err)
	}
	must(child.Close())
	var out bytes.Buffer
	if err := childView.Display(&out); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed-node view error = %v", err)
	}
	must(view.SetMutable())
	if got := show(view); got != "(-5, 6]" {
		t.Fatalf("detached scratch interval = %q", got)
	}
	must(root.Close())
	must(view.Close())
	must(childView.Close())
	must(other.Close())
	must(result.Close())
	must(value.Close())
	must(scratchUpper.Close())
	must(scratchLower.Close())
	must(m.Close())
}
