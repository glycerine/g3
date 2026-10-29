package api

import "testing"

func TestAPINumeralDecimalStringRational(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	third := f.pin(f.c.Numeral("1/3", f.pin(f.c.RealSort())))
	for _, test := range []struct {
		precision uint32
		want      string
	}{{0, "0.?"}, {1, "0.3?"}, {5, "0.33333?"}} {
		got, err := f.c.NumeralDecimalString(third, test.precision)
		astTestMust(t, err)
		if got != test.want {
			t.Fatalf("precision %d: got %q want %q", test.precision, got, test.want)
		}
	}
}
