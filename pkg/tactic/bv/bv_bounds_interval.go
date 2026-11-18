package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp interval operations.

import (
	"github.com/glycerine/g3/pkg/math/interval"
)

// bvBoundsRange is an inclusive range of uint64 values: the source interval
// holds uint64_t bounds (sz <= 64).
type bvBoundsRange struct{ lo, hi uint64 }
type bvBoundsSet struct {
	width  uint32
	ranges []bvBoundsRange
	tight  bool
}

func bvBoundsSingleton(width uint32, v uint64) bvBoundsSet {
	return bvBoundsSet{width: width, ranges: []bvBoundsRange{{v, v}}, tight: true}
}
func bvBoundsRangeSet(width uint32, lo, hi uint64) bvBoundsSet {
	if lo > hi {
		return bvBoundsSet{width: width, tight: true}
	}
	return bvBoundsSet{width: width, ranges: []bvBoundsRange{{lo, hi}}, tight: true}
}
func bvBoundsLooseRangeSet(width uint32, lo, hi uint64) bvBoundsSet {
	b := bvBoundsRangeSet(width, lo, hi)
	b.tight = false
	return b
}
func (b bvBoundsSet) singleton() (uint64, bool) {
	v, ok := b.modularInterval()
	if ok && v.IsSingleton() {
		return v.Lower(), true
	}
	return 0, false
}
func (b bvBoundsSet) empty() bool { return len(b.ranges) == 0 }
func (b bvBoundsSet) intersect(o bvBoundsSet) bvBoundsSet {
	if b.empty() || o.empty() || b.width != o.width {
		return bvBoundsSet{width: b.width, tight: b.tight && o.tight}
	}
	a, aok := b.modularInterval()
	z, zok := o.modularInterval()
	if !aok || !zok {
		return bvBoundsSet{width: b.width}
	}
	var result interval.ModularInterval
	used, err := a.Intersect(z, &result)
	if err != nil || !used {
		return bvBoundsSet{width: b.width, tight: b.tight && o.tight}
	}
	return bvBoundsFromModularInterval(&result)
}
func (b bvBoundsSet) normalize() bvBoundsSet {
	if len(b.ranges) < 2 {
		return b
	}
	for i := 0; i < len(b.ranges); i++ {
		for j := i + 1; j < len(b.ranges); j++ {
			if b.ranges[j].lo < b.ranges[i].lo {
				b.ranges[i], b.ranges[j] = b.ranges[j], b.ranges[i]
			}
		}
	}
	out := bvBoundsSet{width: b.width, tight: b.tight}
	for _, r := range b.ranges {
		n := len(out.ranges)
		// a gap: hi + 1 < r.lo, without wrapping at the uint64 maximum
		if n == 0 || out.ranges[n-1].hi < r.lo && r.lo-out.ranges[n-1].hi > 1 {
			out.ranges = append(out.ranges, r)
			continue
		}
		if r.hi > out.ranges[n-1].hi {
			out.ranges[n-1].hi = r.hi
		}
	}
	return out
}
func (b bvBoundsSet) implies(o bvBoundsSet) bool {
	if b.width != o.width {
		return false
	}
	if b.empty() {
		return true
	}
	if o.empty() {
		return false
	}
	a, aok := b.modularInterval()
	z, zok := o.modularInterval()
	if !aok || !zok {
		return false
	}
	result, err := a.Implies(z)
	return err == nil && result
}
func (b bvBoundsSet) equal(o bvBoundsSet) bool {
	if b.width != o.width || b.empty() || o.empty() {
		return b.width == o.width && b.empty() && o.empty() && b.tight == o.tight
	}
	a, aok := b.modularInterval()
	z, zok := o.modularInterval()
	if !aok || !zok {
		return false
	}
	result, err := a.Equal(z)
	return err == nil && result
}
func (b bvBoundsSet) complement() bvBoundsSet {
	if b.empty() {
		return bvBoundsRangeSet(b.width, 0, bvBoundsMaximum(b.width))
	}
	v, ok := b.modularInterval()
	if !ok {
		return bvBoundsSet{width: b.width}
	}
	var result interval.ModularInterval
	used, err := v.Negate(&result)
	if err != nil || !used {
		return bvBoundsSet{width: b.width, tight: true}
	}
	return bvBoundsFromModularInterval(&result)
}

// sourceComplement follows bv_bounds_tactic.cpp::interval::negate, whose
// loose intervals complement to the full set and whose tight intervals use
// the source's inclusive endpoints directly. negate builds those results
// with the three-argument interval constructor (tight = false), so a negated
// bound is loose: intersecting it with a stored tight bound changes the
// tightness and pushes an undo record.
func (b bvBoundsSet) sourceComplement() bvBoundsSet {
	maximum := bvBoundsMaximum(b.width)
	if !b.tight {
		return bvBoundsRangeSet(b.width, 0, maximum)
	}
	if b.empty() || b.full() {
		return bvBoundsSet{width: b.width, tight: true}
	}
	if len(b.ranges) == 2 {
		// The two stored ranges are the wrapped interval [l..max] U [0..h].
		// Normalized, they are [0..h] U [l..max] with h + 1 < l.
		return bvBoundsLooseRangeSet(b.width, b.ranges[0].hi+1, b.ranges[1].lo-1)
	}
	r := b.ranges[0]
	if r.lo == 0 {
		return bvBoundsLooseRangeSet(b.width, r.hi+1, maximum)
	}
	if r.hi == maximum {
		return bvBoundsLooseRangeSet(b.width, 0, r.lo-1)
	}
	return bvBoundsSet{width: b.width, ranges: []bvBoundsRange{
		{lo: 0, hi: r.lo - 1},
		{lo: r.hi + 1, hi: maximum},
	}}
}
