package api

import (
	"math"
	"testing"
)

// AUDIT.md api_03-F10: D writes doubles as std::ostream does, and a high
// byte is escaped from its unsigned value (the fork's ll_escaped).
func TestZ3LogDoubleAndHighByteEscape(t *testing.T) {
	for _, tc := range []struct {
		value float64
		want  string
	}{{math.Inf(1), "inf"}, {math.Inf(-1), "-inf"}, {math.NaN(), "nan"}, {math.Copysign(math.NaN(), -1), "-nan"}, {0.1, "0.1"}, {1234567, "1.23457e+06"}} {
		if got := z3LogDouble(tc.value); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.value, got, tc.want)
		}
	}
	if got := z3LogEscape("a\xc3b"); got != "a\\195b" {
		t.Fatalf("escape %q", got)
	}
}
