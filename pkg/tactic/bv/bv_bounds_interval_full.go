package bv

// Source: src/math/interval/mod_interval.h:39-41 and
// src/ast/rewriter/bv_bounds_base.h:244-247,331-333.

import "math/big"

func (b bvBoundsSet) full() bool {
	b = b.normalize()
	if b.width == 0 || len(b.ranges) != 1 || b.ranges[0].lo.Sign() != 0 {
		return false
	}
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(b.width)), big.NewInt(1))
	return b.ranges[0].hi.Cmp(maximum) == 0
}
