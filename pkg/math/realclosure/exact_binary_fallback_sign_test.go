package realclosure

import "testing"

func TestExactBinaryFallbackDeterminesNonzeroSigns(t *testing.T) {
	f := newExactBinaryFallbackFixture(t, true)
	negativeAlpha, err := f.base.store.negateRankedRetainedValue(f.alpha, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		p    []*realValue
		want int32
	}{
		{[]*realValue{f.one, f.alpha}, 1},
		{[]*realValue{f.minusOne, negativeAlpha}, -1},
	} {
		got, err := f.base.store.exactSignAtBinaryBound(tc.p, f.alpha.root.owner.intervals, &f.oneBound, true)
		if err != nil || got != tc.want {
			t.Fatal(got, tc.want, err)
		}
	}
	f.releaseValues(negativeAlpha)
}
