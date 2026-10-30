package base_test

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestUint64TextCanonicalDecimal(t *testing.T) {
	for value, want := range map[uint64]string{0: "0", 9: "9", 10: "10", math.MaxUint64: "18446744073709551615"} {
		if got := base.Uint64Text(value); got != want {
			t.Fatal(value, got, want)
		}
	}
}
