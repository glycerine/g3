package subpaving

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestRationalIntervalLiveNodeBoundsAndOwnedCopy(t *testing.T) {
	budget := arena.NewBudget(1 << 24)
	m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	root, err := NewRationalNode(m, 1, nil)
	must(err)
	view, err := NewRationalInterval(m)
	must(err)
	owned, err := NewRationalInterval(m)
	must(err)
	result, err := NewRationalInterval(m)
	must(err)
	lo, hi := numeral.NewRational(), numeral.NewRational()
	t.Cleanup(func() {
		for _, v := range []*RationalInterval{result, owned, view} {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := root.Close(); err != nil {
			t.Error(err)
		}
		for _, v := range []*numeral.Rational{&lo, &hi} {
			if err := m.Del(v); err != nil {
				t.Error(err)
			}
		}
		if err := m.CloseChecked(); err != nil {
			t.Error(err)
		}
		if budget.Used() != 0 {
			t.Error("retained rational node budget", budget.Used())
		}
	})
	if err := view.SetConstant(root, 1); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("out-of-range variable: %v", err)
	}
	must(view.SetConstant(root, 0))
	must(view.SetFrom(view))
	var shown bytes.Buffer
	must(view.Display(&shown))
	if shown.String() != "(-oo, +oo)" {
		t.Fatal("initial unbounded view", shown.String())
	}
	must(m.SetString(&lo, "-3/2"))
	must(m.SetInt32(&hi, 2))
	must(root.PushBound(0, &lo, true, true))
	must(root.PushBound(0, &hi, false, false))
	loBound, hiBound, err := root.Bounds(0)
	must(err)
	resolvedLo, resolvedHi, err := view.resolved()
	must(err)
	if resolvedLo != &loBound.value || resolvedHi != &hiBound.value {
		t.Fatal("node endpoints were copied")
	}
	shown.Reset()
	must(view.Display(&shown))
	if shown.String() != "(-3/2, 2]" {
		t.Fatal("live finite view", shown.String())
	}
	must(view.CheckInvariant())
	contains, err := view.ContainsZero()
	must(err)
	if !contains {
		t.Fatal("crossing view excludes zero")
	}
	if err := view.Reset(); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("borrowed mutation: %v", err)
	}
	must(owned.SetFrom(view))
	must(result.Mul(view, owned))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(-3, 4]" {
		t.Fatal("mixed live view product", shown.String())
	}
	must(m.SetInt32(&lo, 0))
	must(root.PushBound(0, &lo, true, true))
	shown.Reset()
	must(view.Display(&shown))
	if shown.String() != "(0, 2]" {
		t.Fatal("replacement bound invisible", shown.String())
	}
	contains, err = view.ContainsZero()
	must(err)
	if contains {
		t.Fatal("open zero bound includes zero")
	}
	shown.Reset()
	must(owned.Display(&shown))
	if shown.String() != "(-3/2, 2]" {
		t.Fatal("owned copy followed node", shown.String())
	}
	must(result.Mul(view, owned))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(-3, 4]" {
		t.Fatal("changed live view product", shown.String())
	}
	must(result.Mul(view, view))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(0, 4]" {
		t.Fatal("positive live view product", shown.String())
	}
	must(result.Neg(view))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "[-2, 0)" {
		t.Fatal("negated live view", shown.String())
	}
	must(result.Add(view, owned))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(-3/2, 4]" {
		t.Fatal("added live view", shown.String())
	}
	must(result.Sub(owned, view))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(-7/2, 2)" {
		t.Fatal("subtracted live view", shown.String())
	}
	must(result.MulScalar(&hi, view))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(0, 4]" {
		t.Fatal("scaled live view", shown.String())
	}
	must(result.DivScalar(view, &hi))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(0, 1]" {
		t.Fatal("divided live view", shown.String())
	}
	must(m.SetInt32(&lo, -2))
	must(result.MulScalar(&lo, view))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "[-4, 0)" {
		t.Fatal("negative scaled live view", shown.String())
	}
	must(result.DivScalar(view, &lo))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "[-1, 0)" {
		t.Fatal("negative divided live view", shown.String())
	}
	must(result.Div(owned, view))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(-oo, +oo)" {
		t.Fatal("open-zero live denominator", shown.String())
	}
	must(m.SetInt32(&lo, 1))
	must(owned.SetFinite(&lo, &hi, false, false))
	must(result.Div(view, owned))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(0, 2]" {
		t.Fatal("live numerator quotient", shown.String())
	}
	if err := view.Neg(owned); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("borrowed result mutation: %v", err)
	}
	must(result.Power(view, 2))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "[0, 4]" {
		t.Fatal("positive live view even power", shown.String())
	}
	must(result.Inv(view))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "[1/2, +oo)" {
		t.Fatal("positive live view inverse", shown.String())
	}
	must(m.SetInt32(&lo, -2))
	must(root.PushBound(0, &lo, true, true))
	must(m.SetInt32(&lo, -3))
	must(m.SetInt32(&hi, 3))
	must(owned.SetFinite(&lo, &hi, false, false))
	must(result.Mul(view, owned))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "[-6, 6]" {
		t.Fatal("mixed live view closed-tie product", shown.String())
	}
	must(result.Power(view, 2))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "[0, 4]" {
		t.Fatal("mixed live view even power", shown.String())
	}
	must(result.Power(view, 3))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(-8, 8]" {
		t.Fatal("mixed live view odd power", shown.String())
	}
	must(result.Power(view, 1))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "(-2, 2]" {
		t.Fatal("live view first power", shown.String())
	}
	must(result.Power(view, 0))
	shown.Reset()
	must(result.Display(&shown))
	if shown.String() != "[0, 1]" {
		t.Fatal("source mixed zero power", shown.String())
	}
	must(m.SetInt32(&hi, 2))
	child, err := NewRationalNode(m, 1, root)
	must(err)
	if err := root.Close(); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("root closed before child: %v", err)
	}
	childView, err := NewRationalInterval(m)
	must(err)
	must(childView.SetConstant(child, 0))
	must(child.PushBound(0, &hi, true, false))
	shown.Reset()
	must(childView.Display(&shown))
	if shown.String() != "[2, 2]" {
		t.Fatal("child replacement view", shown.String())
	}
	must(childView.Close())
	must(child.Close())
	must(view.SetMutable())
	must(m.SetInt32(&lo, 0))
	must(view.SetFinite(&lo, &hi, false, false))
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := view.SetConstant(root, 0); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed node binding: %v", err)
	}
}
