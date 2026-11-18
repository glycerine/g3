package numeral

import (
	"math"
	"testing"
)

// TestHardwareFloatDirectedDeepUnderflow pins products far below the
// smallest subnormal: the source's FPU rounds them away from zero under the
// matching directed mode. The former math/big rounding returned zero here
// because Rat.Float64 reports such values as exactly representable.
func TestHardwareFloatDirectedDeepUnderflow(t *testing.T) {
	m := NewHardwareFloatManager()
	minSub := math.Float64frombits(1)
	rows := []struct {
		mode ArbitraryFloatRoundingMode
		a, b float64
		want uint64
	}{
		{ArbitraryFloatTowardPositive, minSub, minSub, 1},
		{ArbitraryFloatTowardNegative, minSub, minSub, 0},
		{ArbitraryFloatTowardNegative, -minSub, minSub, 0x8000000000000001},
		{ArbitraryFloatTowardPositive, -minSub, minSub, 0x8000000000000000},
		{ArbitraryFloatTowardZero, minSub, minSub, 0},
		{ArbitraryFloatNearestEven, minSub, minSub, 0},
	}
	for _, row := range rows {
		x, y, out := HardwareFloat{row.a}, HardwareFloat{row.b}, HardwareFloat{}
		if e := m.Mul(row.mode, &x, &y, &out); e != nil {
			t.Fatal(e)
		}
		if got := math.Float64bits(out.value); got != row.want {
			t.Fatalf("mode %v: %g * %g = %#x, want %#x", row.mode, row.a, row.b, got, row.want)
		}
	}
	x, y, out := HardwareFloat{minSub}, HardwareFloat{math.MaxFloat64}, HardwareFloat{}
	if e := m.Div(ArbitraryFloatTowardPositive, &x, &y, &out); e != nil || math.Float64bits(out.value) != 1 {
		t.Fatalf("minsub/max toward +inf = %#x: %v", math.Float64bits(out.value), e)
	}
}
