package realclosure

import (
	"strings"
	"testing"
)

func TestRetainedDecimalValueAlgebraicSigns(t *testing.T) {
	f := newRankedValueFixture(t, true)
	positive := decimalText(t, f, f.alpha, 8)
	if !strings.HasPrefix(positive, "1.41") {
		t.Fatal(positive)
	}
	negativeAlpha, err := f.base.store.negateRankedRetainedValue(f.alpha, true)
	if err != nil {
		t.Fatal(err)
	}
	negative := decimalText(t, f, negativeAlpha, 8)
	if !strings.HasPrefix(negative, "-1.41") {
		t.Fatal(negative)
	}
	f.releaseValues(negativeAlpha)
}
