package idgen

// Source: src/util/id_gen.h:25-85; src/util/vector.h.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// Generator assigns monotonically increasing IDs and reuses released IDs in
// LIFO order. The free list is owned by the generator; copying a generator
// after first use aliases that storage.
type Generator struct {
	next        uint32
	free        []uint32
	outOfMemory func() bool
}

func New(start uint32, outOfMemory func() bool) Generator {
	return Generator{next: start, outOfMemory: outOfMemory}
}

// Reserve ensures that at least needed released IDs can be retained.
func (g *Generator) Reserve(needed uint32) error {
	if uint64(needed) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(needed) > cap(g.free) {
		// Grow geometrically: the AST manager reserves its live-node count on
		// every registration, so exact-size growth copied the list per node.
		n := int(needed)
		if c := cap(g.free) + cap(g.free)/2; c > n && uint64(c) <= uint64(^uint(0)>>1) {
			n = c
		}
		next := make([]uint32, len(g.free), n)
		copy(next, g.free)
		g.free = next
	}
	return nil
}

// Mk returns the most recently recycled ID, or a fresh ID when none is free.
func (g *Generator) Mk() uint32 {
	if len(g.free) == 0 {
		id := g.next
		g.next++
		return id
	}
	last := len(g.free) - 1
	id := g.free[last]
	g.free = g.free[:last]
	return id
}

func (g *Generator) Recycle(id uint32) error {
	if g.outOfMemory != nil && g.outOfMemory() {
		return nil
	}
	if uint64(len(g.free)) >= math.MaxUint32 {
		return arena.ErrBounds
	}
	g.free = append(g.free, id)
	return nil
}

func (g *Generator) Reset(start uint32) {
	g.next = start
	g.free = g.free[:0]
}

func (g *Generator) Cleanup(start uint32) {
	g.next = start
	g.free = nil
}

func (g *Generator) Range() uint32     { return g.next }
func (g *Generator) FreeCapacity() int { return cap(g.free) }
func (g *Generator) FreeLen() int      { return len(g.free) }
func (g *Generator) IsOutOfMemory() bool {
	return g.outOfMemory != nil && g.outOfMemory()
}

// FreeIDs returns a borrowed, read-only view for owner diagnostics.
func (g *Generator) FreeIDs() []uint32 { return g.free }

// SetNextID supports the source repro/debug path. It finds the first ID at or
// above id that is absent from the free list without consuming that list.
func (g *Generator) SetNextID(id uint32) uint32 {
	g.next = id
	for {
		found := false
		for _, free := range g.free {
			if free == g.next {
				found = true
				break
			}
		}
		if !found {
			return g.next
		}
		g.next++
	}
}

// Snapshot copies the generator state, including the free-list order, so a
// later RestoreTransient can undo IDs that were only borrowed meanwhile.
func (g *Generator) Snapshot() Generator {
	return Generator{next: g.next, free: append([]uint32(nil), g.free...), outOfMemory: g.outOfMemory}
}

// RestoreTransient returns g to s when every ID handed out since the snapshot
// has been recycled again: the free list then holds exactly s's free IDs plus
// the fresh IDs in [s.next, g.next). It reports whether g was restored; when
// some ID is still in use g is left unchanged.
func (g *Generator) RestoreTransient(s Generator) bool {
	if g.next < s.next || len(g.free) != len(s.free)+int(g.next-s.next) {
		return false
	}
	want := make(map[uint32]bool, len(s.free))
	for _, id := range s.free {
		want[id] = true
	}
	for _, id := range g.free {
		if id >= s.next && id < g.next {
			continue
		}
		if !want[id] {
			return false
		}
		delete(want, id)
	}
	if len(want) != 0 {
		return false
	}
	seen := make(map[uint32]bool, int(g.next-s.next))
	for _, id := range g.free {
		if id >= s.next && id < g.next {
			if seen[id] {
				return false
			}
			seen[id] = true
		}
	}
	if len(seen) != int(g.next-s.next) {
		return false
	}
	g.next = s.next
	g.free = append(g.free[:0], s.free...)
	return true
}
