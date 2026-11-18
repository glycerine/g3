package simplex

import (
	"testing"
)

// Source simplex_def.h set_value/update_value: a basic column's self-row
// cancels the requested delta. Nonbasic updates still move dependent bases.
func TestSimplexBasicSeedPreservesCanonicalRow(t *testing.T) {
	s := newTestSolver(nil)
	s.SetValue(1, testNI(rat(3, 1), rat(-2, 1)))
	_, err := s.AddInt64Row(2, []uint32{2, 1}, []int64{2, -3})
	if err != nil {
		t.Fatal(err)
	}
	before := testValue(s, 2)
	s.SetValue(2, testNI(rat(100, 1), rat(7, 1)))
	after := testValue(s, 2)
	if icmp(after, before) != 0 {
		t.Fatal("seeding a basic column changed its row value", itext(after), itext(before))
	}
	seed := testNI(rat(4, 1), rat(6, 1))
	s.SetValue(1, seed)
	expected := iclone(seed)
	if err := expected.Mul(optQ(), rat(3, 2)); err != nil {
		t.Fatal(err)
	}
	after = testValue(s, 2)
	if icmp(after, expected) != 0 {
		t.Fatal("nonbasic seed did not move its dependent base", itext(after), itext(expected))
	}
}
