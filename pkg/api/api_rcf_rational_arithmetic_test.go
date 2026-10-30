package api

import "testing"

func TestAPIRCFRationalArithmetic(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	a, b := f.rational("3/2"), f.rational("2/3")
	results := map[string]*RCFNumber{
		"13/6": f.keep(f.context.RCFAdd(a, b)),
		"5/6":  f.keep(f.context.RCFSub(a, b)),
		"1":    f.keep(f.context.RCFMul(a, b)),
		"9/4":  f.keep(f.context.RCFDiv(a, b)),
	}
	for want, value := range results {
		if got := f.text(value); got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

func TestAPIRCFRationalUnary(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	a := f.rational("-2/3")
	if got := f.text(f.keep(f.context.RCFNeg(a))); got != "2/3" {
		t.Fatal(got)
	}
	if got := f.text(f.keep(f.context.RCFInv(a))); got != "-3/2" {
		t.Fatal(got)
	}
	if got := f.text(f.keep(f.context.RCFPower(a, 3))); got != "-8/27" {
		t.Fatal(got)
	}
}
