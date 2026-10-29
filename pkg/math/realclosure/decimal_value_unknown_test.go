package realclosure

import (
	"bytes"
	"testing"
)

func TestRetainedDecimalValueSignDeterminedUnknown(t *testing.T) {
	f := newMultiRootExactFixture(t, true)
	for _, value := range f.values {
		sign, err := f.f.store.determineAnyRetainedValueSign(value, true)
		if err != nil || sign == 0 {
			continue
		}
		want := "?"
		if sign < 0 {
			want = "-?"
		}
		var out bytes.Buffer
		if err := f.f.store.displayValueDecimal(&out, value, f.f.bounds, 8); err != nil || out.String() != want {
			t.Fatal(out.String(), want, err)
		}
		return
	}
	t.Fatal("fixture supplied no nonzero sign-determined value")
}
