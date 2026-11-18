package ast

import (
	"testing"

	"github.com/glycerine/insdict"
)

// astTableLCG is the generator of the fork's TST(insdict) golden vectors
// (src/test/insdict.cpp).
type astTableLCG struct{ x uint64 }

func (l *astTableLCG) next() uint64 {
	l.x = l.x*6364136223846793005 + 1442695040888963407
	return l.x >> 33
}

func astTableFNV(h, v uint64) uint64 {
	for i := 0; i < 8; i++ {
		h ^= (v >> (8 * i)) & 0xff
		h *= 1099511628211
	}
	return h
}

// The canonical table keeps insdict's structure: on the golden operation
// streams of the fork's TST(insdict) (which the Go insdict v0.14.6
// produced) it reports the same capacity, entry count and size after every
// operation, and iterates in the same order as an insdict.Dict.
func TestASTCanonicalTableFollowsInsdict(t *testing.T) {
	type change struct{ op, cap int }
	for _, g := range []struct {
		seed, keySpace, ops uint64
		cap, live, entries  int
		hash                uint64
		changes             []change
	}{
		{1, 50, 4000, 128, 34, 63, 0x8179c5baef7d449c, []change{{0, 8}, {5, 16}, {14, 32}, {47, 64}, {109, 128}}},
		{2, 3000, 20000, 8192, 1771, 4575, 0x739290284483937, []change{{0, 8}, {6, 16}, {13, 32}, {39, 64}, {66, 128}, {149, 256}, {298, 512}, {617, 1024}, {1338, 2048}, {2989, 4096}, {8147, 8192}}},
		{3, 100000, 30000, 32768, 15377, 15900, 0x5e97b9897cc41b48, []change{{0, 8}, {8, 16}, {13, 32}, {29, 64}, {69, 128}, {152, 256}, {281, 512}, {567, 1024}, {1152, 2048}, {2314, 4096}, {4618, 8192}, {9368, 16384}}},
	} {
		m := &ASTManager{}
		d := insdict.NewDict[int, bool]()
		r := &astTableLCG{g.seed}
		h := uint64(14695981039346656037)
		ref := func(k int) astNodeRef { return astNodeRef{slot: uint32(k) + 1, generation: 7} }
		var changes []change
		prev := 0
		for i := 0; i < int(g.ops); i++ {
			k := int(r.next() % g.keySpace)
			if r.next()%10 < 6 {
				if d.Put(k, true) {
					m.table.add(astTableHash(uint32(k)), ref(k), 0, false)
				}
			} else {
				found := d.Del(k)
				if e := m.tableEraseCaptured(ref(k), uint32(k)); (e == nil) != found {
					t.Fatal("erase disagrees with insdict", g.seed, i, k, e, found)
				}
			}
			// The fork's stream also calls pack(false) here; it never fires.
			r.next()
			h = astTableFNV(h, uint64(m.table.capacity()))
			h = astTableFNV(h, uint64(len(m.table.entries)))
			h = astTableFNV(h, uint64(m.table.size))
			if c := int(m.table.capacity()); c != prev {
				prev = c
				if len(changes) < len(g.changes) {
					changes = append(changes, change{i, c})
				}
			}
		}
		got := m.tableEntries()
		j := 0
		for k := range d.All() {
			if j >= len(got) || got[j] != ref(k) {
				t.Fatal("iteration order differs from insdict", g.seed, j)
			}
			h = astTableFNV(h, uint64(k))
			j++
		}
		if j != len(got) {
			t.Fatal("entry count differs from insdict", g.seed, j, len(got))
		}
		if int(m.table.capacity()) != g.cap || int(m.table.size) != g.live || len(m.table.entries) != g.entries || h != g.hash {
			t.Fatalf("seed %d: capacity %d size %d entries %d hash %#x", g.seed, m.table.capacity(), m.table.size, len(m.table.entries), h)
		}
		if len(changes) != len(g.changes) {
			t.Fatal("capacity changes", g.seed, changes)
		}
		for i := range changes {
			if changes[i] != g.changes[i] {
				t.Fatal("capacity changes", g.seed, changes)
			}
		}
	}
}
