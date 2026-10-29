package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

type exactBinaryFallbackFixture struct {
	*rankedValueFixture
	oneBound  numeral.BinaryRational
	halfBound numeral.BinaryRational
	halfAlpha *realValue
	minusOne  *realValue
}

func newExactBinaryFallbackFixture(t *testing.T, usePrem bool) *exactBinaryFallbackFixture {
	t.Helper()
	f := newRankedValueFixture(t, usePrem)
	half, err := f.base.store.newRational(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	halfAlpha, err := f.base.store.multiplyRankedRetainedValues(f.alpha, half, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	minusOne, err := f.base.store.newRational(-1, 1)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &exactBinaryFallbackFixture{
		rankedValueFixture: f,
		oneBound:           numeral.NewBinaryInt(1),
		halfBound:          numeral.NewBinaryIntPower(1, 1),
		halfAlpha:          halfAlpha,
		minusOne:           minusOne,
	}
	t.Cleanup(func() { f.releaseValues(half, halfAlpha, minusOne) })
	return fixture
}
