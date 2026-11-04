package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestArbitraryIntervalLiveRootBoundViewAndOwnedCopy(t *testing.T) {
	f := newArbitraryClauseFixture(t)
	root, err := f.context.Root()
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewArbitraryInterval(f.numbers)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := v.SetFinite(&f.four, &f.five, true, false); err != nil {
		t.Fatal(err)
	}
	if err := v.SetConstant(root, uint32(len(f.context.variables))); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("unknown root variable: %v", err)
	}
	if err := v.SetConstant(root, f.x); err != nil {
		t.Fatal(err)
	}
	if err := v.SetFrom(v); err != nil {
		t.Fatalf("constant self-assignment must be a no-op: %v", err)
	}
	if _, _, err := v.endpoints(); err != nil || !v.lowerInf || !v.upperInf {
		t.Fatalf("initial unbounded node view: %v, %v, %v", v.lowerInf, v.upperInf, err)
	}
	if err := f.context.PublishBound(root, f.x, &f.four, true, false, true); err != nil {
		t.Fatal(err)
	}
	lo, hi, err := v.endpoints()
	if err != nil || v.lowerInf || !v.upperInf || lo != &root.lower[f.x].value || hi != &v.upper {
		t.Fatalf("live lower view: %p, %p, %v", lo, hi, err)
	}
	if err := f.context.PublishBound(root, f.x, &f.five, false, false, true); err != nil {
		t.Fatal(err)
	}
	lo, hi, err = v.endpoints()
	if err != nil || v.lowerInf || v.upperInf || lo != &root.lower[f.x].value || hi != &root.upper[f.x].value {
		t.Fatalf("live two-sided view: %p, %p, %v", lo, hi, err)
	}
	if err := v.Reset(); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("borrowed interval mutation: %v", err)
	}
	copyView, err := NewArbitraryInterval(f.numbers)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := copyView.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := copyView.SetFrom(v); err != nil {
		t.Fatal(err)
	}
	if err := f.context.PublishBound(root, f.x, &f.five, true, false, true); err != nil {
		t.Fatal(err)
	}
	lo, _, err = v.endpoints()
	if err != nil || lo != &root.lower[f.x].value {
		t.Fatalf("view follows replacement bound: %p, %v", lo, err)
	}
	copiedLower, _, err := copyView.endpoints()
	if err != nil || copiedLower == lo {
		t.Fatalf("owned copy remains independent: %p, %p, %v", copiedLower, lo, err)
	}
	equal, err := f.numbers.Equal(copiedLower, &f.four)
	if err != nil || !equal {
		t.Fatalf("owned lower retains former value: %v, %v", equal, err)
	}
	if err := v.SetMutable(); err != nil {
		t.Fatal(err)
	}
	lo, hi, err = v.endpoints()
	if err != nil || v.lowerInf || v.upperInf || !v.lowerOpen || v.upperOpen {
		t.Fatalf("detached arbitrary view flags: %v, %v, %v, %v", v.lowerInf, v.upperInf, v.lowerOpen, err)
	}
	equal, err = f.numbers.Equal(lo, &f.four)
	if err != nil || !equal {
		t.Fatalf("detached arbitrary lower scratch value: %v, %v", equal, err)
	}
	equal, err = f.numbers.Equal(hi, &f.five)
	if err != nil || !equal {
		t.Fatalf("detached arbitrary upper scratch value: %v, %v", equal, err)
	}
	if err := v.SetFinite(&f.four, &f.five, false, false); err != nil {
		t.Fatal(err)
	}
	if v.constantNode != nil || v.lowerInf || v.upperInf {
		t.Fatal("detached mutable interval")
	}
}
