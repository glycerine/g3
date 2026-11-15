package api

import (
	"strings"
	"testing"
)

func TestAPIRCFRationalDisplayAndComponents(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	value := f.rational("-22/7")
	decimal, err := f.context.RCFDecimalString(value, 8)
	if err != nil || !strings.HasPrefix(decimal, "-3.142857") {
		t.Fatal(decimal, err)
	}
	numerator, denominator, err := f.context.RCFNumeratorDenominator(value)
	f.keep(numerator, err)
	f.keep(denominator, err)
	if f.text(numerator) != "-22" || f.text(denominator) != "7" {
		t.Fatal(f.text(numerator), f.text(denominator))
	}
}

func TestAPIRCFSmallIntMinValue(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	value := f.keep(f.context.RCFSmallInt(-2147483648))
	if got := f.text(value); got != "-2147483648" {
		t.Fatal(got)
	}
}

// AUDIT math_realclosure_01-F7: clean_denominators_core multiplies crosswise
// only when the two cleaned denominators differ, keeps the denominator
// positive, and handles non-rational denominators.
func TestAPIRCFNumeratorDenominatorTranscendental(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	pi := f.keep(f.context.RCFPi())
	pi1 := f.keep(f.context.RCFAdd(pi, f.rational("1")))
	expect := func(label string, value *RCFNumber, wantNum, wantDen string) {
		t.Helper()
		numerator, denominator, err := f.context.RCFNumeratorDenominator(value)
		f.keep(numerator, err)
		f.keep(denominator, err)
		if gotNum, gotDen := f.text(numerator), f.text(denominator); gotNum != wantNum || gotDen != wantDen {
			t.Fatalf("%s: num %q den %q, want num %q den %q", label, gotNum, gotDen, wantNum, wantDen)
		}
	}
	expect("1/(pi-4)", f.keep(f.context.RCFInv(f.keep(f.context.RCFSub(pi, f.rational("4"))))), "-1", "-1*pi + 4")
	half := f.keep(f.context.RCFDiv(pi, f.rational("2")))
	numerator := f.keep(f.context.RCFAdd(half, f.rational("1")))
	denominator := f.keep(f.context.RCFAdd(pi, f.rational("1/2")))
	expect("(pi/2+1)/(pi+1/2)", f.keep(f.context.RCFDiv(numerator, denominator)), "pi + 2", "2*pi + 1")
	roots, err := f.context.RCFRoots([]*RCFNumber{f.rational("-2"), f.rational("0"), f.rational("1")})
	if err != nil || len(roots) != 2 {
		t.Fatal(len(roots), err)
	}
	for _, root := range roots {
		f.keep(root, nil)
	}
	product := f.keep(f.context.RCFMul(roots[1], f.keep(f.context.RCFInv(pi1))))
	expect("r/(pi+1)", product, f.text(roots[1]), "pi + 1")
}
