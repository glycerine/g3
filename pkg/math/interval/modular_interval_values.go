package interval

// Source: src/tactic/bv/bv_bounds_tactic.cpp:35-60.

func (v *ModularInterval) Size() uint32 {
	if v == nil {
		return 0
	}
	return v.size
}

func (v *ModularInterval) Tight() bool { return v != nil && v.tight }

func (v *ModularInterval) Lower() uint64 {
	if v == nil {
		return 0
	}
	return v.lower
}

func (v *ModularInterval) Upper() uint64 {
	if v == nil {
		return 0
	}
	return v.upper
}

func (v *ModularInterval) set(lower, upper uint64, tight bool) {
	v.lower, v.upper, v.tight = lower, upper, tight
}
