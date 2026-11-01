package bitutil

import "testing"

func TestNextPowerOfTwo32SourceBoundaries(t *testing.T) {
	for _, test := range []struct {
		input uint32
		want  uint32
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 4},
		{7, 8},
		{8, 8},
		{9, 16},
		{1 << 30, 1 << 30},
		{(1 << 30) + 1, 1 << 31},
		{1 << 31, 1 << 31},
		{(1 << 31) + 1, 0},
		{^uint32(0), 0},
	} {
		if got := NextPowerOfTwo32(test.input); got != test.want {
			t.Errorf("NextPowerOfTwo32(%d) = %d, want %d", test.input, got, test.want)
		}
	}
}
