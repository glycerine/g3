package api

import "testing"

func TestSMTContextFloatingPointNumeralClasses(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.FloatingPointSingleSort())
	nan := f.pin(f.c.FloatingPointNaN(sort))
	positiveInfinity := f.pin(f.c.FloatingPointInfinity(sort, false))
	negativeZero := f.pin(f.c.FloatingPointZero(sort, true))
	tests := []struct {
		name  string
		query func() (bool, error)
		want  bool
	}{
		{"nan", func() (bool, error) { return f.c.FloatingPointNumeralIsNaN(nan) }, true},
		{"infinity", func() (bool, error) { return f.c.FloatingPointNumeralIsInfinite(positiveInfinity) }, true},
		{"zero", func() (bool, error) { return f.c.FloatingPointNumeralIsZero(negativeZero) }, true},
		{"positive infinity", func() (bool, error) { return f.c.FloatingPointNumeralIsPositive(positiveInfinity) }, true},
		{"negative zero", func() (bool, error) { return f.c.FloatingPointNumeralIsNegative(negativeZero) }, true},
		{"nan not positive", func() (bool, error) { return f.c.FloatingPointNumeralIsPositive(nan) }, false},
		{"infinity not normal", func() (bool, error) { return f.c.FloatingPointNumeralIsNormal(positiveInfinity) }, false},
		{"zero not subnormal", func() (bool, error) { return f.c.FloatingPointNumeralIsSubnormal(negativeZero) }, false},
	}
	for _, test := range tests {
		got, err := test.query()
		if err != nil || got != test.want {
			t.Fatalf("%s: %v, %v", test.name, got, err)
		}
	}
	variable := f.pin(f.c.Constant("fp-class-variable", sort))
	if _, err := f.c.FloatingPointNumeralIsZero(variable); err == nil {
		t.Fatal("symbolic value accepted as numeral")
	}
}
