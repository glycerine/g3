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
