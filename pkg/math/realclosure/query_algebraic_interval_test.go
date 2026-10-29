package realclosure

import "testing"

func TestAlgebraicMetadataDefiningInterval(t *testing.T) {
	f := newRankedValueFixture(t, true)
	query, algebraic, err := f.base.store.queryAlgebraicInterval(f.alpha)
	if err != nil || !algebraic {
		t.Fatal(algebraic, query, err)
	}
	interval := f.alpha.root.isolating
	if query.LowerInfinite != interval.lowerInf || query.UpperInfinite != interval.upperInf {
		t.Fatal("infinity flags", query, interval.lowerInf, interval.upperInf)
	}
	var lower, upper *realValue
	if !interval.lowerInf {
		lower, err = f.base.store.ownBinaryBoundValue(interval.numbers, &interval.lower)
	}
	if err == nil && !interval.upperInf {
		upper, err = f.base.store.ownBinaryBoundValue(interval.numbers, &interval.upper)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]*realValue{{query.Lower, lower}, {query.Upper, upper}} {
		equal, compareErr := f.base.store.structurallyEqualValues(pair[0], pair[1])
		if compareErr != nil || !equal {
			t.Fatal(equal, compareErr)
		}
	}
	if query.LowerOpen != interval.lowerOpen || query.UpperOpen != interval.upperOpen {
		t.Fatal(query.LowerOpen, query.UpperOpen, interval.lowerOpen, interval.upperOpen)
	}
	f.releaseValues(lower, upper)
	if err := f.base.store.releaseAlgebraicIntervalQuery(&query); err != nil {
		t.Fatal(err)
	}
}
