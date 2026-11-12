package rel

import (
	"slices"
	"testing"
)

func TestUMapOrder(t *testing.T) {
	cases := []struct{ in, want []uint32 }{
		{[]uint32{3, 0, 2}, []uint32{0, 2, 3}},
		{[]uint32{1, 9}, []uint32{1, 9}},   // 9 collides with 1, probes to slot 2
		{[]uint32{9, 1}, []uint32{9, 1}},   // 9 takes slot 1, 1 probes to slot 2
		{[]uint32{7, 15}, []uint32{15, 7}}, // 15 wraps to slot 0
		// Seven keys fit in 8 slots (6*4 > 8*3 is false), 9 first.
		{[]uint32{9, 1, 2, 3, 4, 5, 6}, []uint32{9, 1, 2, 3, 4, 5, 6}},
		// The 8th insertion (7*4 > 8*3) grows the table to 16 slots, moving
		// the entries in table order; 9 then lands in its own slot.
		{[]uint32{9, 1, 2, 3, 4, 5, 6, 7}, []uint32{1, 2, 3, 4, 5, 6, 7, 9}},
	}
	for _, c := range cases {
		if got := uMapOrder(c.in); !slices.Equal(got, c.want) {
			t.Errorf("uMapOrder(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
