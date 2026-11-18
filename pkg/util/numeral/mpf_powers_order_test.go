package numeral

import "testing"

// AUDIT.md util_numeral_03-F2: u_map iteration follows core_hashtable's
// identity-hash slots and its growth at 3/4 load.
func TestUMapTableOrder(t *testing.T) {
	keys := []uint32{9, 1, 17, 3}
	// capacity 8: 9->1, 1->2 (probe), 17->3 (probe), 3->4 (probe).
	got := uMapTableOrder(keys)
	want := []int{0, 1, 2, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatal(got)
		}
	}
	// Seven inserts grow the table to 16 before the seventh: keys
	// 0..6 then 8 land at 0..6 and 8, and 20 at 4 collides forward.
	keys = []uint32{20, 0, 1, 2, 3, 5, 6}
	got = uMapTableOrder(keys)
	// capacity 8 holds 20@4,0@0,1@1,2@2,3@3,5@5; the seventh insert
	// (size 6: 24 > 24 is false) stays in capacity 8: 6@6.
	want = []int{1, 2, 3, 4, 0, 5, 6}
	for i := range want {
		if got[i] != want[i] {
			t.Fatal(got)
		}
	}
	keys = []uint32{20, 0, 1, 2, 3, 5, 6, 7}
	got = uMapTableOrder(keys)
	// The eighth insert sees size 7 (28 > 24) and doubles to 16: the old
	// table order 0,1,2,3,20,5,6 is moved, 20 to slot 4, then 7@7.
	want = []int{1, 2, 3, 4, 0, 5, 6, 7}
	for i := range want {
		if got[i] != want[i] {
			t.Fatal(got)
		}
	}
}
