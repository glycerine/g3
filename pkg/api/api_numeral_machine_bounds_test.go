package api

import "testing"

func TestAPINumeralMachineExtractorBounds(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	integerSort := f.pin(f.c.IntegerSort())
	tooPositive := f.pin(f.c.Numeral("4294967296", integerSort))
	tooNegative := f.pin(f.c.Numeral("-2147483649", integerSort))
	tooLargeSmall := f.pin(f.c.Numeral("9223372036854775808", integerSort))
	fraction := f.pin(f.c.Numeral("3/2", f.pin(f.c.RealSort())))
	if _, matched, err := f.c.NumeralUint32(tooPositive); err != nil || matched {
		t.Fatal("uint32 overflow", matched, err)
	}
	if _, matched, err := f.c.NumeralInt32(tooNegative); err != nil || matched {
		t.Fatal("int32 overflow", matched, err)
	}
	if _, matched, err := f.c.NumeralInt32(fraction); err != nil || matched {
		t.Fatal("fractional int32", matched, err)
	}
	if _, _, matched, err := f.c.NumeralSmall(tooLargeSmall); err != nil || matched {
		t.Fatal("small rational overflow", matched, err)
	}
	if _, matched, err := f.c.NumeralUint32(tooNegative); err != nil || matched {
		t.Fatal("negative uint32", matched, err)
	}
}
