package api

import (
	"errors"
	"testing"
)

func TestAPIRCFRationalRejectsForeignAndClosedValues(t *testing.T) {
	left, right := newRCFFixture(t), newRCFFixture(t)
	a, b := left.rational("1"), right.rational("2")
	if _, err := left.context.RCFAdd(a, b); !errors.Is(err, ErrRCFNumberOwner) {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	right.values = nil
	if _, err := right.context.RCFNeg(b); !errors.Is(err, ErrRCFNumberOwner) {
		t.Fatal(err)
	}
	right.close()
	left.close()
}

func TestAPIRCFRationalZeroDivisionAndInverseFail(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	one, zero := f.rational("1"), f.rational("0")
	if _, err := f.context.RCFDiv(one, zero); err == nil {
		t.Fatal("division by zero succeeded")
	}
	if _, err := f.context.RCFInv(zero); err == nil {
		t.Fatal("inverse of zero succeeded")
	}
}

func TestAPIRCFRationalZeroPowerAndInputOwnership(t *testing.T) {
	f := newRCFFixture(t)
	base := f.rational("-98765432101234567890/7")
	result := f.keep(f.context.RCFPower(base, 0))
	if got := f.text(result); got != "1" {
		t.Fatalf("zero power = %q, want 1", got)
	}
	if err := base.Close(); err != nil {
		t.Fatal(err)
	}
	f.values = f.values[1:]
	if got := f.text(result); got != "1" {
		t.Fatalf("result changed after closing its input: %q", got)
	}
	f.close()
}
