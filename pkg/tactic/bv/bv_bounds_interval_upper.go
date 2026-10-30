package bv

// Source: src/ast/rewriter/bv_bounds_base.h:188-193.

import "math/big"

func (b bvBoundsSet) nonWrappedUpper() (*big.Int, bool) {
	v, ok := b.modularInterval()
	if !ok || v.IsWrapped() {
		return nil, false
	}
	return v.Upper(), true
}
