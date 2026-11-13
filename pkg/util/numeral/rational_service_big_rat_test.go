package numeral

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// SetBigRat must store exactly what SetString stores for the same value:
// small, word-boundary, multi-limb, negative and fractional rationals.
func TestRationalServiceSetBigRatMatchesSetString(t *testing.T) {
	cases := []string{
		"0", "1", "-1", "2147483647", "2147483648", "-2147483648", "-2147483649",
		"4294967295", "4294967296", "9223372036854775807", "9223372036854775808",
		"-9223372036854775809", "18446744073709551616",
		"123456789012345678901234567890123456789012345678901234567890",
		"-98765432109876543210987654321098765432109876543210",
		"1/3", "-7/2", "4294967296/4294967297",
		"123456789012345678901234567890/98765432109876543210987654321",
		"-340282366920938463463374607431768211457/18446744073709551629",
	}
	s, e := NewRationalService(serviceConfig(arena.NewBudget(1 << 25)))
	serviceMust(t, e)
	for _, text := range cases {
		r, ok := new(big.Rat).SetString(text)
		if !ok {
			t.Fatalf("bad case %q", text)
		}
		viaString := serviceValue(t, s, r.RatString())
		got, e := s.NewValue()
		serviceMust(t, e)
		done, e := s.SetBigRat(&got, r)
		serviceMust(t, e)
		if !done {
			t.Fatalf("%s: SetBigRat declined", text)
		}
		want, e := s.String(&viaString)
		serviceMust(t, e)
		have, e := s.String(&got)
		serviceMust(t, e)
		if want != have {
			t.Fatalf("%s: SetBigRat %s, SetString %s", text, have, want)
		}
		if got.Raw.Num.IsSmall() != viaString.Raw.Num.IsSmall() || got.Raw.Den.IsSmall() != viaString.Raw.Den.IsSmall() {
			t.Fatalf("%s: representation differs (small num %v/%v, small den %v/%v)", text,
				got.Raw.Num.IsSmall(), viaString.Raw.Num.IsSmall(), got.Raw.Den.IsSmall(), viaString.Raw.Den.IsSmall())
		}
		serviceMust(t, s.CleanupValue(&got))
		serviceMust(t, s.CleanupValue(&viaString))
	}
}
