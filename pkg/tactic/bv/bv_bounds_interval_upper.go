package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp:188-193.

func (b bvBoundsSet) nonWrappedUpper() (uint64, bool) {
	v, ok := b.modularInterval()
	if !ok || v.IsWrapped() {
		return 0, false
	}
	return v.Upper(), true
}
