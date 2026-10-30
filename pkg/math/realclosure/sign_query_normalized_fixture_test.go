package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

type normalizedSignQueryFixture struct {
	*realFunctionFixture
	polynomial []*realValue
	x          []*realValue
	positive   *BinaryInterval
	zero       numeral.BinaryRational
}

func newNormalizedSignQueryFixture(t *testing.T) *normalizedSignQueryFixture {
	t.Helper()
	f := &normalizedSignQueryFixture{realFunctionFixture: newRealFunctionFixture(t)}
	negativeAlpha, err := f.store.negateValue(f.alpha)
	mustRealFunction(t, err)
	f.own(negativeAlpha)
	f.polynomial = []*realValue{negativeAlpha, nil, f.one} // x² - alpha
	f.x = []*realValue{nil, f.one}
	f.zero = numeral.NewBinaryRational()
	mustRealFunction(t, f.intervals.SetInt32(&f.zero, 0))
	f.positive, err = NewBinaryInterval(f.intervals)
	mustRealFunction(t, err)
	mustRealFunction(t, f.positive.SetFinite(&f.zero, &f.hi, false, false))
	t.Cleanup(func() {
		mustRealFunction(t, f.positive.Close())
		mustRealFunction(t, f.intervals.Del(&f.zero))
	})
	return f
}
