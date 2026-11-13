package idgen

import (
	"math"
	"testing"
)

// Added tests: src/util/id_gen.h has no dedicated assertion suite. These finite
// expectations preserve source recycling, resource checks and debug ID resets.
func TestIDGeneratorSourceOrder(t *testing.T) {
	outOfMemory := false
	g := New(10, func() bool { return outOfMemory })
	if g.Mk() != 10 || g.Mk() != 11 || g.Range() != 12 {
		t.Fatal("fresh IDs")
	}
	g.Recycle(10)
	g.Recycle(11)
	if g.Mk() != 11 || g.Mk() != 10 || g.Mk() != 12 {
		t.Fatal("source LIFO recycle")
	}
	outOfMemory = true
	g.Recycle(12)
	if g.Mk() != 13 {
		t.Fatal("OOM recycle must be ignored")
	}
	outOfMemory = false
	g.Recycle(13)
	g.Reset(40)
	if g.Mk() != 40 || len(g.free) != 0 {
		t.Fatal("reset")
	}
	g.Recycle(40)
	g.Cleanup(50)
	if g.free != nil || g.Mk() != 50 {
		t.Fatal("cleanup")
	}
}
func TestIDGeneratorDebugAndWrap(t *testing.T) {
	g := New(0, nil)
	for _, v := range []uint32{3, 4, 6} {
		g.Recycle(v)
	}
	if got := g.SetNextID(3); got != 5 || g.Range() != 5 {
		t.Fatal(got)
	}
	if got := g.FreeIDs(); len(got) != 3 || got[0] != 3 || got[1] != 4 || got[2] != 6 {
		t.Fatal(got)
	}
	if g.Mk() != 6 || g.Mk() != 4 || g.Mk() != 3 || g.Mk() != 5 {
		t.Fatal("debug range should leave free IDs intact")
	}
	g.Reset(math.MaxUint32)
	if g.Mk() != math.MaxUint32 || g.Mk() != 0 {
		t.Fatal("source unsigned wrap")
	}
	g.Recycle(1)
	g.Recycle(1)
	if g.Mk() != 1 || g.Mk() != 1 {
		t.Fatal("source does not deduplicate recycle")
	}
}
func TestIDGeneratorFiniteUniqueness(t *testing.T) {
	g := New(0, nil)
	live := map[uint32]bool{}
	state := uint32(1)
	for i := 0; i < 3000; i++ {
		state = state*1664525 + 1013904223
		if len(live) < 64 && (state&3) != 0 {
			id := g.Mk()
			if live[id] {
				t.Fatal("duplicate live ID")
			}
			live[id] = true
			continue
		}
		// Pick deterministically by value, independent of Go map traversal.
		for id := uint32(0); id < g.Range(); id++ {
			if live[id] {
				delete(live, id)
				g.Recycle(id)
				break
			}
		}
	}
	if got := testing.AllocsPerRun(100, func() { id := g.Mk(); g.Recycle(id) }); got != 0 {
		t.Fatal("warm ID recycle allocated", got)
	}
}

func TestIDGeneratorReserveGrowsGeometrically(t *testing.T) {
	g := New(0, nil)
	for n := uint32(1); n <= 1000; n++ {
		if e := g.Reserve(n); e != nil || g.FreeCapacity() < int(n) {
			t.Fatal("reserve", n, e, g.FreeCapacity())
		}
	}
	if c := g.FreeCapacity(); c > 2000 {
		t.Fatal("over-reserved", c)
	}
	grows := 0
	last := g.FreeCapacity()
	for n := uint32(1001); n <= 100000; n++ {
		if e := g.Reserve(n); e != nil {
			t.Fatal(e)
		}
		if g.FreeCapacity() != last {
			grows++
			last = g.FreeCapacity()
		}
	}
	if grows > 20 {
		t.Fatal("reserve reallocated per call", grows)
	}
}

func TestIDGeneratorRestoreTransient(t *testing.T) {
	g := New(0, nil)
	for i := 0; i < 5; i++ {
		g.Mk()
	}
	_ = g.Recycle(3)
	_ = g.Recycle(1)
	s := g.Snapshot()
	a, b, c := g.Mk(), g.Mk(), g.Mk() // 1, 3, then fresh 5
	_ = g.Recycle(a)
	_ = g.Recycle(c)
	_ = g.Recycle(b)
	if !g.RestoreTransient(s) || g.Range() != 5 || len(g.FreeIDs()) != 2 || g.FreeIDs()[0] != 3 || g.FreeIDs()[1] != 1 {
		t.Fatal("transient IDs not restored", g.Range(), g.FreeIDs())
	}
	s = g.Snapshot()
	g.Mk()
	if g.RestoreTransient(s) || g.Range() != 5 || len(g.FreeIDs()) != 1 {
		t.Fatal("live ID restored", g.Range(), g.FreeIDs())
	}
}
