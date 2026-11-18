package bv

// Source: src/math/interval/mod_interval.h:21-230 and
// src/tactic/bv/bv_bounds_tactic.cpp:28-153.

import (
	"github.com/glycerine/g3/pkg/math/interval"
)

// bvBoundsMaximum is uMaxInt(sz) for sz <= 64.
func bvBoundsMaximum(width uint32) uint64 {
	if width == 0 {
		return 0
	}
	return ^uint64(0) >> (64 - width)
}

func (b bvBoundsSet) modularInterval() (*interval.ModularInterval, bool) {
	b = b.normalize()
	if b.width == 0 || b.width > 64 || len(b.ranges) == 0 || len(b.ranges) > 2 {
		return nil, false
	}
	maximum := bvBoundsMaximum(b.width)
	for _, r := range b.ranges {
		if r.lo > r.hi || r.hi > maximum {
			return nil, false
		}
	}
	lower, upper := b.ranges[0].lo, b.ranges[0].hi
	if len(b.ranges) == 2 {
		if b.ranges[0].lo != 0 || b.ranges[1].hi != maximum {
			return nil, false
		}
		lower, upper = b.ranges[1].lo, b.ranges[0].hi
	}
	v, err := interval.NewModularInterval(lower, upper, b.width, b.tight)
	return v, err == nil
}

func bvBoundsFromModularInterval(v *interval.ModularInterval) bvBoundsSet {
	if v == nil {
		return bvBoundsSet{}
	}
	b := bvBoundsSet{width: v.Size(), tight: v.Tight()}
	lower, upper := v.Lower(), v.Upper()
	if v.IsWrapped() {
		b.ranges = []bvBoundsRange{
			{lo: 0, hi: upper},
			{lo: lower, hi: bvBoundsMaximum(v.Size())},
		}
	} else {
		b.ranges = []bvBoundsRange{{lo: lower, hi: upper}}
	}
	return b
}
