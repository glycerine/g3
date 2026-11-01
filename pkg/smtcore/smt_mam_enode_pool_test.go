package smtcore

import "testing"

func TestSMTMAMEnodePoolLIFOResetAndGrowth(t *testing.T) {
	var pool smtMAMEnodePool

	first := make([]*SMTEnode, 2)
	first[0] = &SMTEnode{}
	second := make([]*SMTEnode, 2)
	second[0] = &SMTEnode{}
	pool.recycle(first)
	pool.recycle(second)
	if pool.free[1][0] != second[0] {
		t.Fatal("recycle cleared source vector contents before reuse")
	}

	reused := pool.take(1)
	if &reused[:cap(reused)][0] != &second[0] {
		t.Fatal("pool did not reuse the most recently returned vector")
	}
	if reused[0] != nil {
		t.Fatal("checkout did not reset the reused vector")
	}

	pool.recycle(reused)
	if len(pool.free) != 2 {
		t.Fatalf("free vector count = %d, want 2", len(pool.free))
	}
	grown := pool.take(3)
	if len(grown) != 3 || cap(grown) < 3 || grown[0] != nil {
		t.Fatalf("larger checkout = len %d cap %d values %v", len(grown), cap(grown), grown)
	}
	if len(pool.free) != 1 {
		t.Fatalf("growth checkout left %d free vectors, want 1", len(pool.free))
	}
}
