package realclosure

import (
	"strings"
	"testing"
)

func TestRetainedDecimalValueRationalAndZero(t *testing.T) {
	f := newRankedValueFixture(t, true)
	if got := decimalText(t, f, nil, 8); got != "0" {
		t.Fatal(got)
	}
	third, err := f.base.store.newRational(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	got := decimalText(t, f, third, 8)
	if !strings.HasPrefix(got, "0.33333333") || !strings.HasSuffix(got, "?") {
		t.Fatal(got)
	}
	f.releaseValues(third)
}
