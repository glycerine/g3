package api

import "testing"

func TestSMTContextNumeralParsesFloatingPointStrings(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.FloatingPointSingleSort())
	for _, test := range []struct {
		text  string
		value float32
	}{{"1.5", 1.5}, {"-0.25", -0.25}, {"1.5p1", 3}} {
		parsed := f.pin(f.c.Numeral(test.text, sort))
		native := f.pin(f.c.FloatingPointNumeralFloat32(test.value, sort))
		found, equal, err := f.m.EqualFPANumerals(parsed, native)
		if err != nil || !found || !equal {
			t.Fatalf("floating numeral %q: found=%v equal=%v err=%v", test.text, found, equal, err)
		}
	}
	if _, err := f.c.Numeral("nan", sort); err == nil {
		t.Fatal("source numeral character validation must reject NaN spelling")
	}
}
