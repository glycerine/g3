package numeral

// Source: src/test/hwf.cpp: bug_set_double, bug_to_rational, bug_is_int.
import (
	"math"
	"math/big"
	"testing"
)

func TestHardwareFloatCPPRegression(t *testing.T) {
	m := NewHardwareFloatManager()
	qm, _ := rationalManager(t, false)
	var value HardwareFloat
	for _, f := range []float64{0.1, 1.1, 11.3, 0} {
		if err := m.SetFloat64(&value, f); err != nil {
			t.Fatal(err)
		}
		if !m.IsRegular(&value) {
			t.Fatalf("%g is not regular", f)
		}
	}
	for _, f := range []float64{0, 1, 1.5, 0.875, -1, -1.5, -0.875, 0.1} {
		if err := m.SetFloat64(&value, f); err != nil {
			t.Fatal(err)
		}
		out := NewRational()
		if err := m.ToRational(&value, qm, &out); err != nil {
			t.Fatal(err)
		}
		got, err := qm.String(&out)
		if err != nil {
			t.Fatal(err)
		}
		if err := qm.Del(&out); err != nil {
			t.Fatal(err)
		}
		want := new(big.Rat).SetFloat64(f).RatString()
		if got != want {
			t.Fatalf("%g: rational %s, want %s", f, got, want)
		}
	}
	const bits = uint64(1077720461)<<32 | uint64(2147483648)
	if err := m.SetFloat64(&value, math.Float64frombits(bits)); err != nil {
		t.Fatal(err)
	}
	if m.IsInteger(&value) {
		t.Fatalf("source regression %#x classified integer", bits)
	}
}
