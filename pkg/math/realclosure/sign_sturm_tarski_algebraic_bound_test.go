package realclosure

// Source: src/math/realclosure/realclosure.cpp:2452-2462, 3881-3920.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestRealClosureAlgebraicConstantAndZeroBoundSigns(t *testing.T) {
	budget := arena.NewBudget(1 << 25)
	service, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	bounds, err := numeral.NewBinaryRationalManager(service.Manager.Z, numeral.BinaryRationalConfig{Diagnostics: service})
	if err != nil {
		t.Fatal(err)
	}
	store := &realValueStore{numbers: service.Manager}
	one, err := store.newRational(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	minusOne, err := store.newRational(-1, 1)
	if err != nil {
		t.Fatal(err)
	}
	third, err := store.newRational(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	half, err := store.newRational(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	negativeThird, err := store.newRational(-1, 3)
	if err != nil {
		t.Fatal(err)
	}
	owner := &algebraicRootOwner{values: store, intervals: bounds}
	makeValue := func(coefficients []*realValue, lo, hi int32) *realValue {
		t.Helper()
		lower, upper := numeral.NewBinaryInt(lo), numeral.NewBinaryInt(hi)
		enclosure, err := NewBinaryInterval(bounds)
		if err != nil {
			t.Fatal(err)
		}
		isolating, err := NewBinaryInterval(bounds)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range []*BinaryInterval{enclosure, isolating} {
			if err := v.SetFinite(&lower, &upper, false, false); err != nil {
				t.Fatal(err)
			}
		}
		root, err := owner.mkAlgebraic(coefficients, enclosure, isolating, nil, -1)
		if err != nil {
			t.Fatal(err)
		}
		value, err := store.newAlgebraicValue(root, one)
		if err != nil {
			t.Fatal(err)
		}
		if err := enclosure.Close(); err != nil {
			t.Fatal(err)
		}
		if err := isolating.Close(); err != nil {
			t.Fatal(err)
		}
		return value
	}
	positive := makeValue([]*realValue{minusOne, one}, 1, 2)
	negative := makeValue([]*realValue{one, one}, -2, -1)
	zero := numeral.NewBinaryInt(0)
	nonzero := numeral.NewBinaryInt(2)
	for _, tc := range []struct {
		name         string
		coefficients []*realValue
		bound        *numeral.BinaryRational
		want         int32
	}{
		{"positive constant", []*realValue{positive}, &nonzero, 1},
		{"negative constant", []*realValue{negative}, &nonzero, -1},
		{"positive zero bound", []*realValue{positive, negative}, &zero, 1},
		{"negative zero bound", []*realValue{negative, positive}, &zero, -1},
		{"positive interval Horner", []*realValue{positive, positive}, &nonzero, 1},
		{"negative interval Horner", []*realValue{negative, negative}, &nonzero, -1},
		{"mixed exact rational", []*realValue{positive, one}, &nonzero, 1},
		{"mixed exact dyadic", []*realValue{positive, half}, &nonzero, 1},
		{"mixed refined rational", []*realValue{positive, third}, &nonzero, 1},
		{"mixed negative rational", []*realValue{negative, negativeThird}, &nonzero, -1},
	} {
		got, err := store.signAtBinaryBound(tc.coefficients, bounds, tc.bound)
		if err != nil || got != tc.want {
			t.Fatalf("%s: sign %d, error %v", tc.name, got, err)
		}
	}
	if _, err := store.signAtBinaryBound([]*realValue{positive, negative}, bounds, &nonzero); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("independent-extension exact fallback", err)
	}
	// The source's initial 24-bit interval for -1/3 overlaps zero after
	// adding this narrow positive algebraic enclosure. Refining the rational
	// coefficient makes the Horner result strictly positive.
	closeLower, closeUpper := numeral.NewBinaryIntPower(357913942, 30), numeral.NewBinaryIntPower(357913943, 30)
	closeInterval, err := NewBinaryInterval(bounds)
	if err != nil {
		t.Fatal(err)
	}
	if err := closeInterval.SetFinite(&closeLower, &closeUpper, false, false); err != nil {
		t.Fatal(err)
	}
	closeRootConstant, err := store.newRational(-715827885, 1<<31)
	if err != nil {
		t.Fatal(err)
	}
	closeRoot, err := owner.mkAlgebraic([]*realValue{closeRootConstant, one}, closeInterval, closeInterval, nil, -1)
	if err != nil {
		t.Fatal(err)
	}
	closeValue, err := store.newAlgebraicValue(closeRoot, one)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := store.rationalApproxInterval(negativeThird, bounds)
	if err != nil {
		t.Fatal(err)
	}
	initialSum, err := NewBinaryInterval(bounds)
	if err != nil {
		t.Fatal(err)
	}
	if err := initialSum.Add(initial, closeInterval); err != nil {
		t.Fatal(err)
	}
	if ambiguous, err := initialSum.ContainsZero(); err != nil || !ambiguous {
		t.Fatal("initial interval should be ambiguous", ambiguous, err)
	}
	oneBound := numeral.NewBinaryInt(1)
	store.maxPrecisionBound = true
	store.maxPrecision = 0
	if got, err := store.signAtBinaryBound([]*realValue{negativeThird, closeValue}, bounds, &oneBound); err != nil || got != 1 {
		t.Fatal("zero refinement limit exact fallback", got, err)
	}
	if negativeThird.rationalInterval == nil {
		t.Fatal("zero refinement limit lost the initial cached approximation")
	}
	for _, pair := range [][2]*numeral.BinaryRational{
		{&negativeThird.rationalInterval.lower, &initial.lower},
		{&negativeThird.rationalInterval.upper, &initial.upper},
	} {
		equal, err := bounds.Equal(pair[0], pair[1])
		if err != nil || !equal {
			t.Fatal("zero limit refined an interval before exact fallback", equal, err)
		}
	}
	store.maxPrecision = 128
	if got, err := store.signAtBinaryBound([]*realValue{negativeThird, closeValue}, bounds, &oneBound); err != nil || got != 1 {
		t.Fatal("refined rational coefficient sign", got, err)
	}
	refined := negativeThird.rationalInterval
	if refined == nil {
		t.Fatal("rational coefficient did not retain its refined interval")
	}
	if got, err := store.signAtBinaryBound([]*realValue{negativeThird, closeValue}, bounds, &oneBound); err != nil || got != 1 || negativeThird.rationalInterval != refined {
		t.Fatal("second sign check rebuilt the rational interval", got, err)
	}
	if err := store.release(closeValue); err != nil {
		t.Fatal(err)
	}
	if err := store.release(closeRootConstant); err != nil {
		t.Fatal(err)
	}
	for _, interval := range []*BinaryInterval{initialSum, initial, closeInterval} {
		if err := interval.Close(); err != nil {
			t.Fatal(err)
		}
	}
	minusTwo, err := store.newRational(-2, 1)
	if err != nil {
		t.Fatal(err)
	}
	minusThreeHalves, err := store.newRational(-3, 2)
	if err != nil {
		t.Fatal(err)
	}
	rootLower, rootUpper := numeral.NewBinaryInt(1), numeral.NewBinaryInt(2)
	rootEnclosure, err := NewBinaryInterval(bounds)
	if err != nil {
		t.Fatal(err)
	}
	if err := rootEnclosure.SetFinite(&rootLower, &rootUpper, false, false); err != nil {
		t.Fatal(err)
	}
	squareRoot, err := owner.mkAlgebraic([]*realValue{minusTwo, nil, one}, rootEnclosure, rootEnclosure, nil, -1)
	if err != nil {
		t.Fatal(err)
	}
	squareRootValue, err := store.newAlgebraicValue(squareRoot, one)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.signAtBinaryBound([]*realValue{minusThreeHalves, squareRootValue}, bounds, &oneBound); err != nil || got != -1 {
		t.Fatal("bisected algebraic coefficient sign", got, err)
	}
	if !squareRoot.interval.upperOpen {
		t.Fatal("nonroot midpoint must become an open upper endpoint")
	}
	exactMidpointRoot, err := owner.mkAlgebraic([]*realValue{minusThreeHalves, one}, rootEnclosure, rootEnclosure, nil, -1)
	if err != nil {
		t.Fatal(err)
	}
	exactMidpointValue, err := store.newAlgebraicValue(exactMidpointRoot, one)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.signAtBinaryBound([]*realValue{minusThreeHalves, exactMidpointValue}, bounds, &oneBound); err != nil || got != 0 {
		t.Fatal("exact algebraic midpoint zero sign", got, err)
	}
	if exactMidpointRoot.interval.lowerOpen || exactMidpointRoot.interval.upperOpen {
		t.Fatal("exact midpoint must close the singleton interval")
	}
	if equal, err := bounds.Equal(&exactMidpointRoot.interval.lower, &exactMidpointRoot.interval.upper); err != nil || !equal {
		t.Fatal("exact midpoint interval", equal, err)
	}
	if err := store.release(exactMidpointValue); err != nil {
		t.Fatal(err)
	}
	if err := store.release(squareRootValue); err != nil {
		t.Fatal(err)
	}
	if err := store.release(minusThreeHalves); err != nil {
		t.Fatal(err)
	}
	if err := store.release(minusTwo); err != nil {
		t.Fatal(err)
	}
	if err := rootEnclosure.Close(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []*realValue{third, negativeThird} {
		approximation, err := store.rationalApproxInterval(value, bounds)
		if err != nil {
			t.Fatal(err)
		}
		width := numeral.NewBinaryRational()
		if err := bounds.Sub(&approximation.upper, &approximation.lower, &width); err != nil {
			t.Fatal(err)
		}
		precise, err := bounds.LessOneDiv2k(&width, 24)
		if err != nil || !precise {
			t.Fatal("source rational interval precision", precise, err)
		}
		lowerInside, err := bounds.LessRational(&approximation.lower, store.numbers, &value.rational)
		if err != nil || !lowerInside {
			t.Fatal("source rational lower enclosure", lowerInside, err)
		}
		upperInside, err := bounds.GreaterRational(&approximation.upper, store.numbers, &value.rational)
		if err != nil || !upperInside {
			t.Fatal("source rational upper enclosure", upperInside, err)
		}
		if err := bounds.Del(&width); err != nil {
			t.Fatal(err)
		}
		if err := approximation.Close(); err != nil {
			t.Fatal(err)
		}
	}
	halfInterval, err := store.rationalApproxInterval(half, bounds)
	if err != nil {
		t.Fatal(err)
	}
	if halfInterval.lowerOpen || halfInterval.upperOpen {
		t.Fatal("exact dyadic interval must be closed")
	}
	if equal, err := bounds.Equal(&halfInterval.lower, &halfInterval.upper); err != nil || !equal {
		t.Fatal("exact dyadic interval endpoints", equal, err)
	}
	if err := halfInterval.Close(); err != nil {
		t.Fatal(err)
	}
	rows := []realPolynomial{
		{coefficients: []*realValue{positive}},
		{coefficients: []*realValue{negative, positive}},
	}
	if got, err := store.signVariationsAtInfinity(rows, true); err != nil || got != 0 {
		t.Fatal("positive infinity signs", got, err)
	}
	if got, err := store.signVariationsAtInfinity(rows, false); err != nil || got != 1 {
		t.Fatal("negative infinity signs", got, err)
	}
	crossLower, crossUpper := numeral.NewBinaryInt(-1), numeral.NewBinaryInt(1)
	if err := positive.functionInterval.SetFinite(&crossLower, &crossUpper, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.signValue(positive); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("ambiguous algebraic interval sign", err)
	}
	for _, value := range []*realValue{positive, negative, one, minusOne, third, half, negativeThird} {
		if err := store.release(value); err != nil {
			t.Fatal(err)
		}
	}
	if negativeThird.rationalInterval != nil {
		t.Fatal("rational value release retained its cached interval")
	}
	if err := bounds.CloseChecked(); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if used := budget.Used(); used != 0 {
		t.Fatal("algebraic sign fixture retained bytes", used)
	}
}
