package bv

// Source: src/ast/rewriter/bv_bounds_base.h interval operations.

import "math/big"

type bvBoundsRange struct{ lo, hi *big.Int }
type bvBoundsSet struct {
	width  uint32
	ranges []bvBoundsRange
}

func bvBI(v *big.Int) *big.Int { return new(big.Int).Set(v) }
func bvBoundsSingleton(width uint32, v *big.Int) bvBoundsSet {
	return bvBoundsSet{width: width, ranges: []bvBoundsRange{{bvBI(v), bvBI(v)}}}
}
func bvBoundsRangeSet(width uint32, lo, hi *big.Int) bvBoundsSet {
	if lo.Cmp(hi) > 0 {
		return bvBoundsSet{width: width}
	}
	return bvBoundsSet{width: width, ranges: []bvBoundsRange{{bvBI(lo), bvBI(hi)}}}
}
func (b bvBoundsSet) singleton() (*big.Int, bool) {
	if len(b.ranges) == 1 && b.ranges[0].lo.Cmp(b.ranges[0].hi) == 0 {
		return bvBI(b.ranges[0].lo), true
	}
	return nil, false
}
func (b bvBoundsSet) empty() bool { return len(b.ranges) == 0 }
func (b bvBoundsSet) intersect(o bvBoundsSet) bvBoundsSet {
	r := bvBoundsSet{width: b.width}
	for _, a := range b.ranges {
		for _, z := range o.ranges {
			lo := a.lo
			if z.lo.Cmp(lo) > 0 {
				lo = z.lo
			}
			hi := a.hi
			if z.hi.Cmp(hi) < 0 {
				hi = z.hi
			}
			if lo.Cmp(hi) <= 0 {
				r.ranges = append(r.ranges, bvBoundsRange{bvBI(lo), bvBI(hi)})
			}
		}
	}
	return r.normalize()
}
func (b bvBoundsSet) normalize() bvBoundsSet {
	if len(b.ranges) < 2 {
		return b
	}
	for i := 0; i < len(b.ranges); i++ {
		for j := i + 1; j < len(b.ranges); j++ {
			if b.ranges[j].lo.Cmp(b.ranges[i].lo) < 0 {
				b.ranges[i], b.ranges[j] = b.ranges[j], b.ranges[i]
			}
		}
	}
	out := bvBoundsSet{width: b.width}
	for _, r := range b.ranges {
		n := len(out.ranges)
		if n == 0 || new(big.Int).Add(out.ranges[n-1].hi, big.NewInt(1)).Cmp(r.lo) < 0 {
			out.ranges = append(out.ranges, bvBoundsRange{bvBI(r.lo), bvBI(r.hi)})
			continue
		}
		if r.hi.Cmp(out.ranges[n-1].hi) > 0 {
			out.ranges[n-1].hi = bvBI(r.hi)
		}
	}
	return out
}
func (b bvBoundsSet) implies(o bvBoundsSet) bool { return b.intersect(o).equal(b) }
func (b bvBoundsSet) equal(o bvBoundsSet) bool {
	b = b.normalize()
	o = o.normalize()
	if b.width != o.width || len(b.ranges) != len(o.ranges) {
		return false
	}
	for i := range b.ranges {
		if b.ranges[i].lo.Cmp(o.ranges[i].lo) != 0 || b.ranges[i].hi.Cmp(o.ranges[i].hi) != 0 {
			return false
		}
	}
	return true
}
func (b bvBoundsSet) complement() bvBoundsSet {
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(b.width)), big.NewInt(1))
	cur := big.NewInt(0)
	out := bvBoundsSet{width: b.width}
	for _, r := range b.normalize().ranges {
		if cur.Cmp(r.lo) < 0 {
			out.ranges = append(out.ranges, bvBoundsRange{bvBI(cur), new(big.Int).Sub(r.lo, big.NewInt(1))})
		}
		cur = new(big.Int).Add(r.hi, big.NewInt(1))
	}
	if cur.Cmp(max) <= 0 {
		out.ranges = append(out.ranges, bvBoundsRange{bvBI(cur), max})
	}
	return out
}
