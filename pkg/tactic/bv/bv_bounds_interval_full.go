package bv

// Source: src/math/interval/mod_interval.h:39-41 and
// src/ast/rewriter/bv_bounds_base.h:244-247,331-333.

func (b bvBoundsSet) full() bool {
	v, ok := b.modularInterval()
	return ok && v.IsFull()
}
