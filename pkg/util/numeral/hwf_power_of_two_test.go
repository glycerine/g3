package numeral

import (
	"errors"
	"math"
	"testing"
)

func TestHardwareFloatPrevPowerOfTwoSourceBoundaries(t *testing.T) {
	core := NewHardwareFloatManager()
	m, err := NewHardwareFloatNumeralManager(core, 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		value float64
		want  uint32
	}{
		{"negative", -1, 0},
		{"negative-zero", math.Copysign(0, -1), 0},
		{"positive-zero", 0, 0},
		{"smallest-subnormal", math.SmallestNonzeroFloat64, 0},
		{"at-source-cutoff", math.Ldexp(1, -52), 0},
		{"above-source-cutoff", math.Ldexp(1, -51), 0},
		{"next-exponent", math.Ldexp(1, -50), 1},
		{"one", 1, 51},
		{"two", 2, 52},
		{"largest-finite", math.MaxFloat64, 1074},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var x HardwareFloat
			if err := core.SetFloat64(&x, tc.value); err != nil {
				t.Fatal(err)
			}
			got, err := m.PrevPowerOfTwo(&x)
			if err != nil || got != tc.want || core.PrevPowerOfTwo(&x) != tc.want {
				t.Fatalf("prev power of two(%g) = %d, %v; want %d", tc.value, got, err, tc.want)
			}
		})
	}
	var nonregular HardwareFloat
	if err := core.MkNaN(&nonregular); err != nil {
		t.Fatal(err)
	}
	if _, err := m.PrevPowerOfTwo(&nonregular); !errors.Is(err, ErrHardwareFloatNonRegular) {
		t.Fatal("non-regular wrapper input", err)
	}
	if _, err := m.PrevPowerOfTwo(nil); !errors.Is(err, ErrNumericOwner) {
		t.Fatal("missing wrapper input", err)
	}
}
