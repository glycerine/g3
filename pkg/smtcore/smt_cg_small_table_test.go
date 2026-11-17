package smtcore

import (
	"fmt"
	"math/rand"
	"testing"
)

// TestSMTCgSmallTableMatchesMap drives the open-addressing congruence table
// and a Go map through the same random insert/overwrite/delete sequence.
func TestSMTCgSmallTableMatchesMap(t *testing.T) {
	m := astTestManager(t)
	nodes := make([]*SMTEnode, 48)
	for i := range nodes {
		nodes[i] = &SMTEnode{owner: smtRelevancyBool(t, m, fmt.Sprintf("cg-small-%d", i))}
	}
	rng := rand.New(rand.NewSource(1))
	var table smtCgSmallTable
	reference := map[[2]*SMTEnode]*SMTEnode{}
	for step := 0; step < 200000; step++ {
		k0 := nodes[rng.Intn(len(nodes))]
		var k1 *SMTEnode
		if rng.Intn(4) != 0 {
			k1 = nodes[rng.Intn(len(nodes))]
		}
		key := [2]*SMTEnode{k0, k1}
		switch rng.Intn(3) {
		case 0:
			v := nodes[rng.Intn(len(nodes))]
			table.set(k0, k1, v)
			reference[key] = v
		case 1:
			table.del(k0, k1)
			delete(reference, key)
		}
		if got := table.get(k0, k1); got != reference[key] {
			t.Fatalf("step %d: get mismatch", step)
		}
		if table.count != len(reference) {
			t.Fatalf("step %d: count %d, want %d", step, table.count, len(reference))
		}
	}
	seen := 0
	table.forEach(func(k0, k1, node *SMTEnode) {
		seen++
		if reference[[2]*SMTEnode{k0, k1}] != node {
			t.Fatal("forEach entry missing from reference")
		}
	})
	if seen != len(reference) {
		t.Fatalf("forEach saw %d entries, want %d", seen, len(reference))
	}
}
